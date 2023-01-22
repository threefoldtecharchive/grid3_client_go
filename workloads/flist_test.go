// Package workloads includes workloads types (vm, zdb, qsfs, public IP, gateway name, gateway fqdn, disk)
package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlistChecksum(t *testing.T) {

	badURL := "flist"
	rightURL := "https://hub.grid.tf/tf-official-apps/base:latest.flist"

	t.Run("test_successful_checksum", func(t *testing.T) {
		_, err := GetFlistChecksum(rightURL)
		assert.NoError(t, err)
	})

	t.Run("test_failed_checksum", func(t *testing.T) {
		_, err := GetFlistChecksum(badURL)
		assert.Error(t, err)
	})
}
