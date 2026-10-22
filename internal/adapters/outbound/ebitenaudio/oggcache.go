package ebitenaudio

import (
	"container/list"
	"errors"
	"io"
)

// oggCacheLimit caps the mono PCM kept from decoded OGG one-shots (about
// 8 minutes of 48 kHz audio); the least recently played sound makes room.
const oggCacheLimit = 24 << 20

type oggKey struct {
	id string
	v  int // index among the id's OGG variants
}

type oggItem struct {
	key oggKey
	pcm []int16
}

// oggCache is an LRU of decoded OGG one-shots, bounded by PCM bytes. It
// is used only from the game loop.
type oggCache struct {
	limit int // bytes of PCM
	bytes int
	ll    *list.List // front = most recently used
	items map[oggKey]*list.Element
}

func newOggCache(limit int) *oggCache {
	return &oggCache{limit: limit, ll: list.New(), items: map[oggKey]*list.Element{}}
}

func (c *oggCache) get(k oggKey) ([]int16, bool) {
	el, ok := c.items[k]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*oggItem).pcm, true
}

// put stores pcm, evicting from the least recently used end. A sound
// larger than the whole cache is not stored.
func (c *oggCache) put(k oggKey, pcm []int16) {
	size := 2 * len(pcm)
	if size > c.limit {
		return
	}
	if el, ok := c.items[k]; ok {
		c.drop(el)
	}
	c.items[k] = c.ll.PushFront(&oggItem{key: k, pcm: pcm})
	c.bytes += size
	for c.bytes > c.limit {
		c.drop(c.ll.Back())
	}
}

func (c *oggCache) drop(el *list.Element) {
	it := c.ll.Remove(el).(*oggItem)
	delete(c.items, it.key)
	c.bytes -= 2 * len(it.pcm)
}

// monoPCM decodes an OGG with decode (48 kHz 16-bit stereo) and mixes
// it down to mono.
func monoPCM(data []byte, decode func([]byte) (io.ReadSeeker, int64, error)) ([]int16, error) {
	s, _, err := decode(data)
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(s)
	if err != nil {
		return nil, err
	}
	frames := len(buf) / 4
	if frames == 0 {
		return nil, errors.New("no samples")
	}
	out := make([]int16, frames)
	for i := range out {
		l := int16(uint16(buf[4*i]) | uint16(buf[4*i+1])<<8)
		r := int16(uint16(buf[4*i+2]) | uint16(buf[4*i+3])<<8)
		out[i] = int16((int(l) + int(r)) / 2)
	}
	return out, nil
}
