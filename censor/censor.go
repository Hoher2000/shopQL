package censor

import (
	"bufio"
	"log"
	"os"
	"strings"
)

var dict map[string]struct{}

func init() {
	dict = map[string]struct{}{}
	f, err := os.Open("/workspaces/codespaces-blank/shopQL/censor/badwords.txt")
	if err != nil {
		log.Printf("censor - open bad words error - %v\n", err)
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		dict[sc.Text()] = struct{}{}
	}
}

func Is(text string) (string, bool) {
	for w := range strings.FieldsSeq(text) {
		if _, ok := dict[strings.ToLower(w)]; ok {
			return w, true
		}
	}
	return "", false
}
