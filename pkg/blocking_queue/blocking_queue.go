package blocking_queue

import "sync"

type Queue[D interface{}] interface {
	PushBack(data D)
	Size() int
	Poll() D
	Pop() (data D, ok bool)
}

type node[D interface{}] struct {
	next *node[D]
	data D
}

type BlockingQueue[D interface{}] struct {
	mutex sync.Mutex
	cond  sync.Cond
	first *node[D]
	last  *node[D]
	size  int
}

func New[D interface{}]() *BlockingQueue[D] {
	return &BlockingQueue[D]{}
}

func (b *BlockingQueue[D]) PushBack(data D) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	newNode := &node[D]{
		data: data,
	}
	if b.last != nil {
		b.last.next = newNode
	}
	b.last = newNode
	if b.first == nil {
		b.first = newNode
	}
	b.size++

	b.cond.Signal()
}

func (b *BlockingQueue[D]) Size() int {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.size
}

func (b *BlockingQueue[D]) Poll() D {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for b.size == 0 {
		b.cond.Wait()
	}
	return b.pop()
}

func (b *BlockingQueue[D]) Pop() (data D, ok bool) {
	var dummy D
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.size == 0 {
		return dummy, false
	}
	return b.pop(), true
}

func (b *BlockingQueue[D]) pop() D {
	curNode := b.first
	b.first = curNode.next
	b.size--
	if b.size == 0 {
		b.last = nil
	}
	return curNode.data
}
