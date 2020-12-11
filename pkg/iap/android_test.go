package iap

import (
	"fmt"
	"testing"
)

func TestGetAndroidList(t *testing.T) {
	results, err := GetAndroidList()

	if err != nil {
		t.Error(err)
	}

	for _, v := range results {
		fmt.Println(v)
	}
}
