package define

import (
	"fmt"
	"strconv"
	"strings"
)

type USBConfig struct {
	Bus       string
	DevNumber string
	Vendor    int
	Product   int
}

func ParseUSBs(usbs []string) ([]USBConfig, error) {
	configs := []USBConfig{}
	for _, str := range usbs {
		if str == "" {
			// Ignore --usb="" as it can be used to reset USBConfigs
			continue
		}

		vals := strings.Split(str, ",")
		if len(vals) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing ',': %s", str)
		}

		left := strings.Split(vals[0], "=")
		if len(left) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing '=': %s", str)
		}

		right := strings.Split(vals[1], "=")
		if len(right) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing '=': %s", str)
		}

		option := left[0] + "_" + right[0]

		switch option {
		case "bus_devnum", "devnum_bus":
			bus, devnumber := left[1], right[1]
			if right[0] == "bus" {
				bus, devnumber = devnumber, bus
			}

			configs = append(configs, USBConfig{
				Bus:       bus,
				DevNumber: devnumber,
			})
		case "vendor_product", "product_vendor":
			vendorStr, productStr := left[1], right[1]
			if right[0] == "vendor" {
				vendorStr, productStr = productStr, vendorStr
			}

			vendor, err := strconv.ParseInt(vendorStr, 16, 0)
			if err != nil {
				return configs, fmt.Errorf("usb: fail to convert vendor of %s: %s", str, err)
			}

			product, err := strconv.ParseInt(productStr, 16, 0)
			if err != nil {
				return configs, fmt.Errorf("usb: fail to convert product of %s: %s", str, err)
			}

			configs = append(configs, USBConfig{
				Vendor:  int(vendor),
				Product: int(product),
			})
		default:
			return configs, fmt.Errorf("usb: fail to parse: %s", str)
		}
	}
	return configs, nil
}
