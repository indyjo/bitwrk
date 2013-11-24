//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013  Jonas Eschenburg <jonas@bitwrk.net>
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <http://www.gnu.org/licenses/>.

package bitwrk

import (
	"fmt"
	"log"
	"os"
)

type Logger interface {
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	New(v ...interface{}) Logger
	Newf(format string, v ...interface{}) Logger
}

type logger struct {
	*log.Logger
}

func (l logger) New(v ...interface{}) Logger {
	flags := l.Flags()
	prefix := l.Prefix()
	if prefix == "" {
		prefix = fmt.Sprint(v...)
	} else {
		prefix = fmt.Sprint(prefix, "/", fmt.Sprint(v...))
	}
	return logger{log.New(os.Stdout, prefix, flags)}
}

func (l logger) Newf(format string, v ...interface{}) Logger {
	flags := l.Flags()
	prefix := l.Prefix()
	if prefix == "" {
		prefix = fmt.Sprintf(format, v...)
	} else {
		prefix = fmt.Sprint(prefix, "/", fmt.Sprintf(format, v...))
	}
	return logger{log.New(os.Stdout, prefix, flags)}
}

var defaultLogger = logger{log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)}

func Root() Logger {
	return defaultLogger
}
