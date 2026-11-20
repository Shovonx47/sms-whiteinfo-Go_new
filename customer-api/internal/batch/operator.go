package batch

import "strings"

// OperatorKeyForPhone maps a Bangladeshi mobile number onto the operator key
// used by sender_operator_gateways (gp, bl, tt, airtel, robi). It returns an
// empty string when the prefix is not recognised.
func OperatorKeyForPhone(phone string) string {
	digits := strings.Builder{}
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	number := digits.String()
	number = strings.TrimPrefix(number, "880")
	if !strings.HasPrefix(number, "0") {
		number = "0" + number
	}
	if len(number) < 3 {
		return ""
	}

	switch number[:3] {
	case "013", "017":
		return "gp"
	case "014", "019":
		return "bl"
	case "015":
		return "tt"
	case "016":
		return "airtel"
	case "018":
		return "robi"
	}

	return ""
}
