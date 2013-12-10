package plist

// Bitmap of characters that must be inside a quoted string
// when written to an old-style property list
// Low bits represent lower characters, and each uint64 represents 64 characters.
var quotable = [4]uint64{
	0x78001385ffffffff,
	0xa800000138000000,
	0xffffffffffffffff,
	0xffffffffffffffff,
}

var whitespace = [4]uint64{
	0x0000000100003f00,
	0x0000000000000000,
	0x0000000000000000,
	0x0000000000000000,
}

func quoteRequired(c uint8) bool {
	return quotable[c/64]&(1<<(c%64)) > 0
	// char 0 is bit 0 in 0
	// char 8 is bit 8 in 0
	// char 65 is bit 0 in 1
}
