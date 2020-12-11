package iap

import (
	"fmt"
	"testing"
)

func TestGetIosList(t *testing.T) {
	results, err := GetIosList()

	if err != nil {
		t.Error(err)
	}

	for _, v := range results {
		fmt.Println(v)
	}
}
