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
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	New(v ...interface{}) Logger
	Newf(format string, v ...interface{}) Logger
}

type logger struct {
	*log.Logger
	context string
}

func (l logger) log(s string) {
	if l.context == "" {
		l.Output(3, s)
	} else {
		l.Output(3, fmt.Sprint("[", l.context, "] ", s))
	}
}

func (l logger) Print(v ...interface{}) {
	l.log(fmt.Sprint(v...))
}

func (l logger) Printf(format string, v ...interface{}) {
	l.log(fmt.Sprintf(format, v...))
}

func (l logger) Println(v ...interface{}) {
	l.log(fmt.Sprintln(v...))
}

func (l logger) New(v ...interface{}) Logger {
	flags := l.Flags()
	context := l.context
	if context == "" {
		context = fmt.Sprint(v...)
	} else {
		context = fmt.Sprint(context, "/", fmt.Sprint(v...))
	}
	return logger{log.New(os.Stdout, l.Prefix(), flags), context}
}

func (l logger) Newf(format string, v ...interface{}) Logger {
	flags := l.Flags()
	context := l.context
	if context == "" {
		context = fmt.Sprintf(format, v...)
	} else {
		context = fmt.Sprint(context, "/", fmt.Sprintf(format, v...))
	}
	return logger{log.New(os.Stdout, l.Prefix(), flags), context}
}

var defaultLogger = logger{log.New(os.Stdout, "", log.LstdFlags), ""}

func Root() Logger {
	return defaultLogger
}
