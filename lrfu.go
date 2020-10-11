// Lee et al's "LRFU (Least Recently/Frequently Used) Replacement Policy: A Spectrum of Block Replacement Policies"
// This is modified (from the original 0...1 (lfu to lru) to (lru to lfu))
package lrfu

import (
	"container/list"
	"math"
)

// Ensure that it's a comparable type. See http://golang.org/ref/spec#Comparison_operators
type Key interface{}

type entry struct {
	key           Key
	value         interface{}
	lastReference uint
	lastCRF       float64
}

// This is not thread-safe, which means it will depend on the parent implementation to do the locking mechanism.
type LRFU struct {
	maxEntries int
	lambda     float64
	OnEvicted  func(key Key, value interface{})
	ll         *list.List
	cache      map[interface{}]*list.Element
	count      uint
	smallest   *list.Element
}

func NewLRFU(maxEntries int, lambda float64) *LRFU {
	return &LRFU{
		maxEntries: maxEntries,
		lambda:     lambda,
		ll:         list.New(),
		cache:      make(map[interface{}]*list.Element),
		count:      0,
	}
}

func (lru *LRFU) Set(key Key, value interface{}) {
	if lru.cache == nil {
		lru.cache = make(map[interface{}]*list.Element)
		lru.ll = list.New()
		lru.count = 0
		lru.smallest = nil
	}

	lru.count++

	if ele, ok := lru.cache[key]; ok {
		lru.ll.MoveToFront(ele)
		ele.Value.(*entry).lastCRF = lru.getWeight(0) + lru.getCRF(ele.Value.(*entry))
		ele.Value.(*entry).lastReference = lru.count
		ele.Value.(*entry).value = value
		lru.restore(ele)

		return
	}

	e := &entry{
		key:           key,
		value:         value,
		lastReference: lru.count,
		lastCRF:       lru.getWeight(0),
	}

	ele := lru.ll.PushFront(e)
	lru.cache[key] = ele
	lru.restore(ele)

	if lru.maxEntries != 0 && lru.ll.Len() > lru.maxEntries {
		lru.RemoveElement()
	}
}

func (lru *LRFU) Get(key Key) (value interface{}, ok bool) {
	if lru.cache == nil {
		return
	}

	lru.count++

	if ele, hit := lru.cache[key]; hit {
		lru.ll.MoveToFront(ele)
		ele.Value.(*entry).lastCRF = lru.getWeight(0) + lru.getCRF(ele.Value.(*entry))
		ele.Value.(*entry).lastReference = lru.count
		lru.restore(ele)
		return ele.Value.(*entry).value, true
	}

	return
}

func (lru *LRFU) restore(ele *list.Element) {
	if lru.smallest == nil {
		lru.smallest = ele
		return
	}

	fe := lru.ll.Front()

	en := ele.Value.(*entry)
	smallest := lru.smallest.Value.(*entry)
	if fe.Value.(*entry).key != en.key {
		if lru.getCRF(en) > lru.getCRF(smallest) {
			*en, *smallest = *smallest, *en
			lru.cache[en.key] = ele
			lru.cache[smallest.key] = lru.smallest
			lru.restore(lru.smallest)
			return
		}

		lru.smallest = ele
	}
	return
}

func (lru *LRFU) getCRF(en *entry) float64 {
	return lru.getWeight(lru.count-en.lastReference) * en.lastCRF
}

func (lru *LRFU) RemoveElement() {
	if lru.cache == nil {
		return
	}
	ele := lru.ll.Back()

	if ele != nil {
		lru.removeElement(ele)
	}
}

func (lru *LRFU) getWeight(v uint) float64 {
	return math.Pow((1 / 2), lru.lambda*float64(v))
}

func (lru *LRFU) removeElement(e *list.Element) {
	if lru.smallest.Value.(*entry).key == e.Value.(*entry).key {
		lru.smallest = nil
	}

	lru.ll.Remove(e)
	kv := e.Value.(*entry)

	delete(lru.cache, kv.key)

	if lru.OnEvicted != nil {
		lru.OnEvicted(kv.key, kv.value)
	}
}

func (lru *LRFU) Len() int {
	if lru.cache == nil {
		return 0
	}
	return lru.ll.Len()
}

func (lru *LRFU) Remove(key Key) (ok bool) {
	if lru.cache == nil {
		return
	}

	if ele, hit := lru.cache[key]; hit {
		lru.removeElement(ele)
		return true
	}

	return false

}

func (lru *LRFU) Clear() {
	if lru.OnEvicted != nil {
		for _, e := range lru.cache {
			kv := e.Value.(*entry)
			lru.OnEvicted(kv.key, kv.value)
		}
	}

	lru.ll = nil
	lru.cache = nil
	lru.count = 0
}
