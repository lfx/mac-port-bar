package main

import (
	"reflect"
	"testing"
)

func TestParseLsof(t *testing.T) {
	output := `COMMAND   PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
python3 9999991 liudas    3u  IPv4 0x4cff84095c90996c      0t0  TCP *:8080 (LISTEN)
node    9999992 liudas   51u  IPv6 0xedb974cc99f0cb72      0t0  TCP *:3000 (LISTEN)
go      9999993 liudas   39u  IPv4 0x528c651d1ab21a39      0t0  TCP 127.0.0.1:51912 (LISTEN)`

	// For the sake of the test, we override getPwdForPid to return a predictable string
	// Wait, we can't easily mock it without refactoring, but for this test it will likely return ""
	// because PIDs are fake/don't exist. That's fine.

	procs, err := parseLsof(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(procs) != 3 {
		t.Fatalf("expected 3 procs, got %d", len(procs))
	}

	expected := []Process{
		{PID: "9999992", Command: "node", Port: "3000", PWD: ""},
		{PID: "9999991", Command: "python3", Port: "8080", PWD: ""},
		{PID: "9999993", Command: "go", Port: "51912", PWD: ""},
	}

	if !reflect.DeepEqual(procs, expected) {
		t.Errorf("expected %+v, got %+v", expected, procs)
	}
}
