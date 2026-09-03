package app

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"newsfall/internal/cache"
	"newsfall/internal/config"
	"newsfall/internal/feed"
	"newsfall/internal/media"
	"newsfall/internal/model"
	"newsfall/internal/platform"
	"newsfall/internal/term"
	"newsfall/internal/ui"
)

// Options configures the interactive runtime. Nil I/O fields use the process
// terminal; empty paths use Newsfall's XDG defaults.
type Options struct {
	ConfigPath string
	DataPath   string
	Demo       bool
	Stdin      *os.File
	Stdout     io.Writer
	Fetcher    *feed.Fetcher
	Loader     *media.Loader
}

type runtimeModel struct {
	interaction *Interaction
	width       int
	height      int
	images      map[string]image.Image
	loading     bool
	errors      []string
	tick        int
	lastRefresh time.Time
	nextRefresh time.Time
}

func (m *runtimeModel) view(now time.Time) ui.State {
	controller := m.interaction.Controller
	return ui.State{
		Width: m.width, Height: m.height, Now: now, Config: controller.Config,
		Columns: controller.Columns, Stream: controller.Articles,
		StreamSelected: controller.StreamSelected, Active: controller.Active,
		Images: m.images, NewIDs: controller.NewIDs,
		Loading: m.loading, Paused: m.interaction.Paused, Help: m.interaction.Help,
		CommandMode: m.interaction.CommandMode, CommandText: m.interaction.CommandText,
		OverlayTitle: m.interaction.OverlayTitle, OverlayLines: m.interaction.OverlayLines,
		Status: m.interaction.Status, Errors: m.errors,
		LastRefresh: m.lastRefresh, NextRefresh: m.nextRefresh, Tick: m.tick,
	}
}

type runner struct {
	runtimeModel
	ctx          context.Context
	configPath   string
	dataPath     string
	demo         bool
	stdin        *os.File
	stdout       io.Writer
	fetcher      *feed.Fetcher
	loader       *media.Loader
	fetchResults chan feed.Result
	imageResults chan imageResult
	requested    map[string]bool
	imageSem     chan struct{}
	lastInput    time.Time
	lastDrift    time.Time
}

type imageResult struct {
	id  string
	img image.Image
	err error
}

// Run starts Newsfall in an alternate terminal screen and blocks until the
// user quits, the context is canceled, or terminal output fails.
func Run(ctx context.Context, options Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	configPath := options.ConfigPath
	if configPath == "" {
		configPath = config.ConfigPath()
	}
	dataPath := options.DataPath
	if dataPath == "" {
		dataPath = config.DataPath()
	}
	controller, status, err := loadController(configPath, dataPath, options.Demo)
	if err != nil {
		return err
	}
	interaction := NewInteraction(controller)
	interaction.Status = status
	width, height := term.CurrentSize(stdin)
	fetcher := options.Fetcher
	if fetcher == nil {
		fetcher = feed.NewFetcher()
	}
	loader := options.Loader
	if loader == nil {
		loader = media.NewLoader()
	}
	now := time.Now()
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	r := &runner{
		runtimeModel: runtimeModel{interaction: interaction, width: width, height: height, images: make(map[string]image.Image)},
		ctx:          childCtx, configPath: configPath, dataPath: dataPath, demo: options.Demo,
		stdin: stdin, stdout: stdout, fetcher: fetcher, loader: loader,
		fetchResults: make(chan feed.Result, 1), imageResults: make(chan imageResult, 32),
		requested: make(map[string]bool), imageSem: make(chan struct{}, 3),
		lastInput: now, lastDrift: now,
	}
	if options.Demo {
		r.lastRefresh = now.Add(-9 * time.Second)
	} else {
		r.nextRefresh = now
	}

	session, err := term.Begin(stdin, stdout)
	if err != nil {
		return err
	}
	defer session.Close()

	keyEvents := make(chan Key, 128)
	go readKeys(childCtx, stdin, keyEvents)

	signals := make(chan os.Signal, 8)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGWINCH)
	defer signal.Stop(signals)

	frameTicker := time.NewTicker(500 * time.Millisecond)
	defer frameTicker.Stop()

	if !options.Demo {
		r.requestRefresh(now)
	}
	r.queueImages()
	if err := r.draw(now); err != nil {
		return err
	}

	for {
		select {
		case <-childCtx.Done():
			return nil
		case key := <-keyEvents:
			now = time.Now()
			r.lastInput = now
			outcome := r.interaction.Handle(key)
			if outcome.Quit {
				return nil
			}
			r.applyOutcome(outcome, now)
			r.queueImages()
			if err := r.draw(now); err != nil {
				return err
			}
		case result := <-r.fetchResults:
			now = time.Now()
			r.acceptFetch(result, now)
			r.queueImages()
			if err := r.draw(now); err != nil {
				return err
			}
		case result := <-r.imageResults:
			if result.err == nil && result.img != nil {
				r.images[result.id] = result.img
				if err := r.draw(time.Now()); err != nil {
					return err
				}
			}
		case sig := <-signals:
			if sig == syscall.SIGWINCH {
				r.width, r.height = term.CurrentSize(stdin)
				if err := r.draw(time.Now()); err != nil {
					return err
				}
				continue
			}
			return nil
		case now = <-frameTicker.C:
			r.tick++
			r.advanceTimers(now)
			if err := r.draw(now); err != nil {
				return err
			}
		}
	}
}

// BuildSnapshot loads cached state without network access or terminal setup.
func BuildSnapshot(configPath, dataPath string, demo bool, width, height int) (ui.State, error) {
	if configPath == "" {
		configPath = config.ConfigPath()
	}
	if dataPath == "" {
		dataPath = config.DataPath()
	}
	controller, status, err := loadController(configPath, dataPath, demo)
	if err != nil {
		return ui.State{}, err
	}
	if width <= 0 {
		width = 150
	}
	if height <= 0 {
		height = 40
	}
	interaction := NewInteraction(controller)
	interaction.Status = status
	now := time.Now()
	model := runtimeModel{interaction: interaction, width: width, height: height, images: map[string]image.Image{}, tick: 4}
	if demo {
		model.lastRefresh = now.Add(-9 * time.Second)
		model.nextRefresh = now.Add(4*time.Minute + 51*time.Second)
	}
	return model.view(now), nil
}

func loadController(configPath, dataPath string, demo bool) (*Controller, string, error) {
	if demo {
		state := ui.DemoState(150, 40)
		controller := NewController(state.Config, state.Stream)
		controller.NewIDs = state.NewIDs
		return controller, "offline demo signal · press ? for controls", nil
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("load Newsfall config: %w", err)
	}
	articles, cacheErr := cache.Load(dataPath)
	controller := NewController(cfg, articles)
	status := fmt.Sprintf("%d cached stories · synchronizing", len(controller.Articles))
	if cacheErr != nil {
		status = "cache unavailable · synchronizing fresh feeds"
	}
	return controller, status, nil
}

func (r *runner) draw(now time.Time) error {
	frame := strings.ReplaceAll(ui.Render(r.view(now)), "\n", "\r\n")
	_, err := io.WriteString(r.stdout, term.HomeSequence+frame+"\x1b[0m")
	return err
}

func (r *runner) applyOutcome(outcome Outcome, now time.Time) {
	if outcome.Reload {
		cfg, err := config.Load(r.configPath)
		if err != nil {
			r.interaction.Status = "reload failed · " + err.Error()
		} else {
			r.interaction.Controller.SetConfig(cfg)
			r.interaction.Status = "configuration reloaded"
			r.nextRefresh = now.Add(config.RefreshDuration(cfg))
			outcome.Refresh = true
		}
	}
	if outcome.SaveConfig {
		if err := config.Save(r.configPath, r.interaction.Controller.Config); err != nil {
			r.interaction.Status = "save failed · " + err.Error()
		} else {
			r.nextRefresh = now.Add(config.RefreshDuration(r.interaction.Controller.Config))
		}
	}
	if outcome.OpenURL != "" {
		if err := platform.OpenURL(outcome.OpenURL); err != nil {
			r.interaction.Status = "open failed · " + err.Error()
		}
	}
	if outcome.Refresh {
		r.requestRefresh(now)
	}
}

func (r *runner) requestRefresh(now time.Time) {
	if r.loading {
		r.interaction.Status = "synchronization already in progress"
		return
	}
	if r.demo {
		r.interaction.Status = "offline demo · live network refresh is disabled"
		return
	}
	feeds := r.interaction.Controller.Config.Feeds
	if len(feeds) == 0 {
		r.interaction.Status = "no feeds configured · use :feed add <url>"
		r.nextRefresh = now.Add(config.RefreshDuration(r.interaction.Controller.Config))
		return
	}
	r.loading = true
	r.interaction.Status = fmt.Sprintf("synchronizing %d feeds", len(feeds))
	r.nextRefresh = now.Add(config.RefreshDuration(r.interaction.Controller.Config))
	definitions := append([]config.Feed(nil), feeds...)
	go func() {
		result := r.fetcher.Fetch(r.ctx, definitions)
		select {
		case r.fetchResults <- result:
		case <-r.ctx.Done():
		}
	}()
}

func (r *runner) acceptFetch(result feed.Result, now time.Time) {
	r.loading = false
	r.errors = r.errors[:0]
	for _, item := range result.Errors {
		r.errors = append(r.errors, item.Error())
	}
	newCount := 0
	if len(result.Articles) > 0 {
		newCount = r.interaction.Controller.Merge(result.Articles)
	}
	r.lastRefresh = now
	r.nextRefresh = now.Add(config.RefreshDuration(r.interaction.Controller.Config))
	r.interaction.Status = fetchStatus(len(r.interaction.Controller.Config.Feeds), result, newCount)
	if err := cache.Save(r.dataPath, r.interaction.Controller.Articles); err != nil {
		r.interaction.Status += " · cache save failed"
	}
}

func (r *runner) advanceTimers(now time.Time) {
	cfg := r.interaction.Controller.Config
	if !r.interaction.Paused && !r.loading && !r.demo && !r.nextRefresh.IsZero() && !now.Before(r.nextRefresh) {
		r.requestRefresh(now)
	}
	if r.interaction.Paused || !cfg.Ambient || r.interaction.CommandMode || r.interaction.Help || len(r.interaction.OverlayLines) > 0 {
		return
	}
	drift := config.DriftDuration(cfg)
	if now.Sub(r.lastInput) >= drift && now.Sub(r.lastDrift) >= drift {
		r.interaction.Controller.Drift()
		r.lastDrift = now
		r.queueImages()
	}
}

func (r *runner) queueImages() {
	if !r.interaction.Controller.Config.Images {
		return
	}
	for _, article := range imageCandidates(r.interaction.Controller, 24) {
		if article.ID == "" || article.ImageURL == "" || r.images[article.ID] != nil || r.requested[article.ID] {
			continue
		}
		r.requested[article.ID] = true
		article := article
		go func() {
			select {
			case r.imageSem <- struct{}{}:
				defer func() { <-r.imageSem }()
			case <-r.ctx.Done():
				return
			}
			img, err := r.loader.Load(r.ctx, article.ImageURL)
			result := imageResult{id: article.ID, img: img, err: err}
			select {
			case r.imageResults <- result:
			case <-r.ctx.Done():
			}
		}()
	}
}

func readKeys(ctx context.Context, stdin *os.File, output chan<- Key) {
	var decoder Decoder
	buffer := make([]byte, 128)
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := stdin.Read(buffer)
		if n > 0 {
			if !emitKeys(ctx, output, decoder.Feed(buffer[:n])) {
				return
			}
		}
		if n == 0 {
			if !emitKeys(ctx, output, decoder.Flush()) {
				return
			}
			// With VMIN=0/VTIME>0, an idle terminal read returns zero bytes.
			// os.File reports that timeout as io.EOF, but the terminal is still
			// open and future keystrokes must continue to be read.
			if err == io.EOF {
				continue
			}
		}
		if err != nil {
			_ = emitKeys(ctx, output, decoder.Flush())
			return
		}
	}
}

func emitKeys(ctx context.Context, output chan<- Key, keys []Key) bool {
	for _, key := range keys {
		select {
		case output <- key:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

func imageCandidates(controller *Controller, limit int) []model.Article {
	if controller == nil || limit <= 0 {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]model.Article, 0, limit)
	add := func(article model.Article) {
		if len(out) >= limit || article.ID == "" || article.ImageURL == "" || seen[article.ID] {
			return
		}
		seen[article.ID] = true
		out = append(out, article)
	}
	if article, ok := controller.SelectedArticle(); ok {
		add(article)
	}
	for _, view := range controller.Columns {
		if len(view.Articles) > 0 {
			selected := normalize(view.Selected, len(view.Articles))
			add(view.Articles[selected])
		}
	}
	if len(controller.Articles) > 0 {
		add(controller.Articles[normalize(controller.StreamSelected, len(controller.Articles))])
	}
	for offset := 1; offset <= 2; offset++ {
		for _, view := range controller.Columns {
			if len(view.Articles) > offset {
				add(view.Articles[normalize(view.Selected+offset, len(view.Articles))])
			}
		}
	}
	for _, article := range controller.Articles {
		add(article)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func fetchStatus(totalFeeds int, result feed.Result, newCount int) string {
	success := totalFeeds - len(result.Errors)
	if success < 0 {
		success = 0
	}
	return fmt.Sprintf("%d/%d feeds · %d %s · %d new · %d %s",
		success, totalFeeds,
		len(result.Articles), plural(len(result.Articles), "story", "stories"),
		newCount,
		len(result.Errors), plural(len(result.Errors), "error", "errors"),
	)
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// FormatErrors produces a compact, display-safe diagnostic used by CLI tools.
func FormatErrors(errors []string) string {
	return strings.Join(errors, "; ")
}
