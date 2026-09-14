package main

import "embed"

//go:embed embed/start.sh embed/innoextract embed/libs.zip embed/config.json build/appicon.png
var bundledAssets embed.FS

func getStartSh() []byte {
	startSh, _ := bundledAssets.ReadFile("embed/start.sh")
	return startSh
}

func getInnoextract() []byte {
	innoextract, _ := bundledAssets.ReadFile("embed/innoextract")
	return innoextract
}

func getLibs() []byte {
	libs, _ := bundledAssets.ReadFile("embed/libs.zip")
	return libs
}

func getOfflineConfig() []byte {
	offlineConfig, _ := bundledAssets.ReadFile("embed/config.json")
	return offlineConfig
}

func getIcon() []byte {
	icon, _ := bundledAssets.ReadFile("build/appicon.png")
	return icon
}