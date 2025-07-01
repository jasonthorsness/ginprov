package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

var (
	ErrWorkerPoolOverCapacity = errors.New("worker pool over capacity")
	ErrGeneratePanic          = errors.New("generate function panicked")
)

type pending struct {
	progressCh chan<- string
	resultCh   chan<- HandleFunc
}

type Server struct {
	pending       map[string][]pending
	workerPool    *WorkerPool
	site          Site
	logger        *slog.Logger
	pw            ProgressWriter
	unsafeHandler HandleFunc
	mu            sync.Mutex
}

func NewServer(
	site Site,
	workerPool *WorkerPool,
	logger *slog.Logger,
	pw ProgressWriter,
	unsafeHandler HandleFunc,
) *Server {
	return &Server{make(map[string][]pending), workerPool, site, logger, pw, unsafeHandler, sync.Mutex{}}
}

//nolint:cyclop
func (s *Server) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slug := r.URL.Path

		if slug == "" {
			slug = IndexSlug
		}

		handleFunc, generateFunc, err := s.site.Handle(slug)
		if err != nil {
			switch {
			case errors.Is(err, ErrUnsafe):
				handleFunc = s.unsafeHandler
			case errors.Is(err, ErrNotFound):
				handleFunc, generateFunc, err = s.site.Handle(NotFoundSlug)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			default:
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if generateFunc == nil {
			err = handleFunc(w)
			if err != nil {
				s.logger.Error("failed to serve file", "slug", slug, "error", err)
				return
			}

			return
		}

		progressCh, resultCh, err := s.singleFlightGenerate(slug, generateFunc) //nolint:contextcheck
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		supportsProgress := strings.HasSuffix(slug, ExtensionHTML)

		if supportsProgress {
			err = handleWithProgress(ctx, w, progressCh, resultCh, s.pw)
		} else {
			err = handleWithoutProgress(ctx, w, handleFunc, resultCh)
		}

		if err != nil {
			s.logger.Error("failed to serve file", "slug", slug, "error", err)
			return
		}
	}
}

func handleWithoutProgress(
	ctx context.Context,
	w http.ResponseWriter,
	handleFunc HandleFunc,
	resultCh <-chan HandleFunc,
) error {
	err := handleFunc(w)
	if err != nil {
		return fmt.Errorf("failed initial handleFunc: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil
	case handleFunc = <-resultCh:
		return handleFunc(w)
	}
}

func handleWithProgress(
	ctx context.Context,
	w http.ResponseWriter,
	progressCh <-chan string,
	resultCh <-chan HandleFunc,
	pw ProgressWriter,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case v := <-progressCh:
			pw.Start(w)
			pw.Chunk(w, v)

			for {
				select {
				case <-ctx.Done():
					return nil
				case v = <-progressCh:
					pw.Chunk(w, v)
				case vv := <-resultCh:
					pw.Finish(w, vv)
					return nil
				}
			}
		case handleFunc := <-resultCh:
			return handleFunc(w)
		}
	}
}

func (s *Server) singleFlightGenerate(
	slug string,
	generateFunc GenerateFunc,
) (<-chan string, <-chan HandleFunc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const progressChannelLength = 4

	progressCh := make(chan string, progressChannelLength)
	resultCh := make(chan HandleFunc, 1)

	p, ok := s.pending[slug]
	s.pending[slug] = append(p, pending{progressCh, resultCh})

	ctx := context.Background()

	if !ok {
		if !DoWork(ctx, s.workerPool, generateFunc, s.generate(slug)) {
			delete(s.pending, slug)
			return nil, nil, ErrWorkerPoolOverCapacity
		}
	}

	return progressCh, resultCh, nil
}

func (s *Server) generate(slug string) func(context.Context, GenerateFunc) {
	return func(ctx context.Context, generateFunc GenerateFunc) {
		var v HandleFunc

		defer func() {
			var err error

			r := recover()
			if r != nil {
				err = fmt.Errorf("%w: %v ", ErrGeneratePanic, r)
			}

			p := func() []pending {
				s.mu.Lock()
				defer s.mu.Unlock()

				p := s.pending[slug]
				delete(s.pending, slug)

				return p
			}()

			if err != nil {
				v = func(w http.ResponseWriter) error {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return nil
				}
			}

			for _, pp := range p {
				if !trySend(pp.resultCh, v) {
					panic("result channel must have capacity")
				}
			}
		}()

		v = generateFunc(ctx, func(progress string) {
			s.mu.Lock()
			defer s.mu.Unlock()

			p := s.pending[slug]
			for _, pp := range p {
				trySend(pp.progressCh, progress)
			}
		})
	}
}
