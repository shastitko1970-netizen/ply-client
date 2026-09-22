package core

import "fmt"

// CoreSlot describes one VPN engine. Only the selected slot is ever started.
// A second core is not downloaded and is not kept in memory "just in case".
type CoreSlot struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Ready bool   `json:"ready"`
	Note  string `json:"note"`
}

func CoreCatalog() []CoreSlot {
	return []CoreSlot{{
		ID:    "xray",
		Title: "Xray",
		Ready: true,
		Note:  "одно ядро в памяти",
	}}
}

func UseCore(id string) error {
	if id == "" || id == "xray" {
		return nil
	}
	return fmt.Errorf("ядро %s не в комплекте: второе не качаем и не держим рядом с Xray", id)
}
