package main

import "embed"

//go:embed embed/credentials.json embed/start.sh embed/innoextract
var bundledAssets embed.FS

func getCreds() []byte {
	creds, _ := bundledAssets.ReadFile("embed/credentials.json")
	return creds
}

func getStartSh() []byte {
	startSh, _ := bundledAssets.ReadFile("embed/start.sh")
	return startSh
}

func getInnoextract() []byte {
	innoextract, _ := bundledAssets.ReadFile("embed/innoextract")
	return innoextract
}
