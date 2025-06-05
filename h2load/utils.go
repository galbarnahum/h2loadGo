package h2load

import (
	"fmt"
	urlpkg "net/url"
	"strings"
	"sync"
)

type IndexedError struct {
	Index int
	Err   error
}

func (e IndexedError) Error() string {
	return fmt.Sprintf("index %d: %v", e.Index, e.Err)
}

func RunConcurrent[A any](items []*A, fn func(*A) error) []IndexedError {
	var wg sync.WaitGroup
	errCh := make(chan IndexedError, len(items)) // buffered

	wg.Add(len(items))
	for i, item := range items {
		go func(idx int, val *A) {
			defer wg.Done()
			if err := fn(val); err != nil {
				errCh <- IndexedError{Index: idx, Err: err}
			}
		}(i, item)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var errs []IndexedError
	for err := range errCh {
		errs = append(errs, err)
	}

	return errs
}

func JoinIndexedErrors(errs []IndexedError) error {
	if len(errs) == 0 {
		return nil
	}
	lines := make([]string, 0, len(errs))
	for _, e := range errs {
		lines = append(lines, e.Error())
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func getHostname(rawURL string) string {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Hostname()
}
