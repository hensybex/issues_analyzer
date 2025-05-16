package fs

import (
	"bufio"
	"os"
	"sync"
)

type cache struct {
	mu   sync.RWMutex
	data map[string][]string
}

var c = cache{data: make(map[string][]string)}

func Line(path string, n int) string {
	c.mu.RLock()
	lines, ok := c.data[path]
	c.mu.RUnlock()

	if !ok {
		file, err := os.Open(path)
		if err != nil {
			return "[could not open source]"
		}
		defer file.Close()
		sc := bufio.NewScanner(file)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		c.mu.Lock()
		c.data[path] = lines
		c.mu.Unlock()
	}

	if n-1 < 0 || n-1 >= len(lines) {
		return "[line out of range]"
	}
	return lines[n-1]
}
