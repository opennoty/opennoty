package waitsignal

import "sync"

type Holder[D interface{}] struct {
	mutex  sync.Mutex
	cond   sync.Cond
	signal bool
	data   D
	err    error
}

func (w *Holder[D]) Signal(d D) {
	w.mutex.Lock()
	w.signal = true
	w.data = d
	w.cond.Broadcast()
	w.mutex.Unlock()
}

func (w *Holder[D]) Throw(err error) {
	w.mutex.Lock()
	w.signal = true
	w.err = err
	w.cond.Broadcast()
	w.mutex.Unlock()
}

func (w *Holder[D]) Reset() {
	w.mutex.Lock()
	w.signal = false
	w.err = nil
	w.mutex.Unlock()
}

func (w *Holder[D]) Wait() (D, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.signal {
		return w.data, w.err
	}
	w.cond.Wait()
	return w.data, w.err
}
