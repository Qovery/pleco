package main

import (
	"github.com/Qovery/pleco/cmd"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
	cmd.Execute()
}
