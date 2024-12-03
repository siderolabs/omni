// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package boards contains helpers for mapping board types to the overlays.
package boards

import (
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// GetOverlay converts board constant to it's overlay.
func GetOverlay(board string) *schematic.Overlay {
	switch board {
	case constants.BoardBananaPiM64:
		return &schematic.Overlay{
			Name:  "bananapi_m64",
			Image: "siderolabs/sbc-allwinner",
		}
	case constants.BoardJetsonNano:
		return &schematic.Overlay{
			Name:  "jetson_nano",
			Image: "siderolabs/sbc-jetson",
		}
	case constants.BoardLibretechAllH3CCH5:
		return &schematic.Overlay{
			Name:  "libretech_all_h3_cc_h5",
			Image: "siderolabs/sbc-allwinner",
		}
	case constants.BoardPine64:
		return &schematic.Overlay{
			Name:  "pine64",
			Image: "siderolabs/sbc-allwinner",
		}
	case constants.BoardRock64:
		return &schematic.Overlay{
			Name:  "bananapi_m64",
			Image: "siderolabs/sbc-allwinner",
		}
	case constants.BoardRockpi4:
		return &schematic.Overlay{
			Name:  "rockpi4",
			Image: "siderolabs/sbc-rockchip",
		}
	case constants.BoardRockpi4c:
		return &schematic.Overlay{
			Name:  "rockpi4c",
			Image: "siderolabs/sbc-rockchip",
		}
	case constants.BoardNanoPiR4S:
		return &schematic.Overlay{
			Name:  "nanopi-r4s",
			Image: "siderolabs/sbc-rockchip",
		}
	case constants.BoardRPiGeneric:
		return &schematic.Overlay{
			Name:  "rpi_generic",
			Image: "siderolabs/sbc-raspberrypi",
		}
	case "turingrk1":
		return &schematic.Overlay{
			Name:  "turingrk1",
			Image: "siderolabs/sbc-rockchip",
		}
	}

	return nil
}
