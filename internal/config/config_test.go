// Package config for handling user configuration
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSave(t *testing.T) {
	testDir := t.TempDir()
	path := filepath.Join(testDir, ".tfgridconfig")
	t.Run("create new config file and write to it", func(t *testing.T) {
		c := Config{
			Mnemonics: "test",
			Network:   "net",
		}
		err := c.Save(path)
		assert.NoError(t, err)
		assert.FileExists(t, path)

		fileContent, err := os.ReadFile(path)
		assert.NoError(t, err)

		configJSON := []byte(`{"mnemonics":"test","network":"net"}`)

		assert.Equal(t, fileContent, configJSON)
	})
	t.Run("overwrite an existing config file", func(t *testing.T) {
		c := Config{
			Mnemonics: "new-test",
			Network:   "new-net",
		}
		err := c.Save(path)
		assert.NoError(t, err)
		assert.FileExists(t, path)

		fileContent, err := os.ReadFile(path)
		assert.NoError(t, err)

		configJSON := []byte(`{"mnemonics":"new-test","network":"new-net"}`)

		assert.Equal(t, fileContent, configJSON)
	})
	t.Run("inavlid path", func(t *testing.T) {
		c := Config{
			Mnemonics: "test",
			Network:   "net",
		}
		err := c.Save("./path/invalid/.config")
		assert.Error(t, err)
	})
}

func TestLoad(t *testing.T) {
	testDir := t.TempDir()
	path := filepath.Join(testDir, ".tfgridconfig")

	f, err := os.Create(path)
	assert.NoError(t, err)
	defer f.Close()

	t.Run("load from a config file", func(t *testing.T) {
		_, err := f.Write([]byte(`{"mnemonics":"test","network":"net"}`))
		assert.NoError(t, err)

		c := Config{}

		err = c.Load(path)
		assert.NoError(t, err)

		assert.Equal(t, c.Mnemonics, "test")
		assert.Equal(t, c.Network, "net")
	})
	t.Run("load from invalid file", func(t *testing.T) {
		c := Config{}

		err := c.Load("./invalid/path/.config")
		assert.Error(t, err)
		assert.Empty(t, c)
	})
	t.Run("load invalid JSON from a file", func(t *testing.T) {
		_, err := f.Write([]byte("invalid json"))
		assert.NoError(t, err)

		c := Config{}

		err = c.Load(path)
		assert.Error(t, err)

		assert.Empty(t, c)
	})
}
