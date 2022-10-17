package utils

import "github.com/go-logr/logr"

func CheckErr(err error, msg string) {
	l := logr.Logger{}
	if err != nil {
		l.Error(err, msg)
	}
}
