//go:build !windows

package main

func openWebShell() bool { return false }

func openBrowserShell() bool { return false }
