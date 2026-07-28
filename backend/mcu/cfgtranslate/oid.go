package cfgtranslate

// NextOID returns the next available OID number from config command lines.
func NextOID(lines []string) int {
	return countOIDs(lines)
}
