package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	err := Init()
	assert.NoError(t, err)
	res, err := Translate("hello world", "auto", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
