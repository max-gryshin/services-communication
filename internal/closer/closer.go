package closer

import (
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/max-gryshin/services-communication/internal/log"
)

type Closer struct {
	sync.Mutex
	once  sync.Once
	done  chan struct{}
	funcs []func() error
}

func New(s ...os.Signal) *Closer {
	c := &Closer{done: make(chan struct{})}
	if len(s) > 0 {
		go func() {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, s...)
			<-ch
			signal.Stop(ch)
			c.CloseAll()
		}()
	}
	return c
}

func (c *Closer) Add(f ...func() error) {
	c.Lock()
	c.funcs = append(c.funcs, f...)
	c.Unlock()
}

func (c *Closer) Wait() {
	<-c.done
}

func (c *Closer) CloseAll() {
	c.once.Do(func() {
		defer close(c.done)

		c.Lock()
		funcs := c.funcs
		c.funcs = nil
		c.Unlock()

		errs := make(chan error, len(funcs))
		for _, f := range funcs {
			go func(f func() error) {
				errs <- f()
			}(f)
		}

		for i := 0; i < cap(errs); i++ {
			err := <-errs
			if err != nil {
				log.Error(fmt.Sprintf("error from func: %v", err.Error()))
			}
		}
	})
}
