// github.com/hduplooy/gorfb project support.go
// Functions to support the network actions
package gorfb

// SetUint64 set 8 bytes at pos in buf to the val (in big endian format)
// A test is done to ensure there are 8 bytes available at pos in the buffer
func SetUint64(buf []byte, pos int, val uint64) {
	if pos+8 > len(buf) {
		return
	}
	for i := 0; i < 8; i++ {
		buf[7-i+pos] = byte(val)
		val >>= 8
	}
}

// SetUint32 set 4 bytes at pos in buf to the val (in big endian format)
// A test is done to ensure there are 4 bytes available at pos in the buffer
func SetUint32(buf []byte, pos int, val uint32) {
	if pos+4 > len(buf) {
		return
	}
	for i := 0; i < 4; i++ {
		buf[3-i+pos] = byte(val)
		val >>= 8
	}
}

// SetUint16 set 2 bytes at pos in buf to the val (in big endian format)
// A test is done to ensure there are 2 bytes available at pos in the buffer
func SetUint16(buf []byte, pos int, val uint16) {
	if pos+2 > len(buf) {
		return
	}
	for i := 0; i < 2; i++ {
		buf[1-i+pos] = byte(val)
		val >>= 8
	}
}

// GetUint64 gets 8 bytes at pos in buf and return it as uint64 (from big endian format)
// A test is done to ensure there are 8 bytes available at pos in the buffer
func GetUint64(buf []byte, pos int) uint64 {
	if pos+8 > len(buf) {
		return 0
	}
	val := uint64(0)
	for i := 0; i < 8; i++ {
		val = (val << 8) + uint64(buf[pos+i])
	}
	return val
}

// GetUint32 gets 4 bytes at pos in buf and return it as uint32 (from big endian format)
// A test is done to ensure there are 4 bytes available at pos in the buffer
func GetUint32(buf []byte, pos int) uint32 {
	if pos+4 > len(buf) {
		return 0
	}
	val := uint32(0)
	for i := 0; i < 4; i++ {
		val = (val << 8) + uint32(buf[pos+i])
	}
	return val
}

// GetUint16 gets 2 bytes at pos in buf and return it as uint16 (from big endian format)
// A test is done to ensure there are 2 bytes available at pos in the buffer
func GetUint16(buf []byte, pos int) uint16 {
	if pos+2 > len(buf) {
		return 0
	}
	val := uint16(0)
	for i := 0; i < 2; i++ {
		val = (val << 8) + uint16(buf[pos+i])
	}
	return val
}
