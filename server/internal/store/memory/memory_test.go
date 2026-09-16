package memory_test

import (
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/storetest"
)

func TestMemoryStoreConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		return memory.New()
	})
}
