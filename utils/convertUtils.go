package utils

import (
	"fmt"
	"strconv"
)

func Int(d *int, s string) error {
	var err error
	*d, err = strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("input data must be int, got - %v\n", s)
	}
	return nil
}
