package main

import "net/textproto"

type FileHeader struct {
	Filename string
	Header textproto.MIMEHeader
}