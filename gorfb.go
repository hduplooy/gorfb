// gorfb project gorfb.go
// Author: Hannes du Plooy
// Revision Date: 26 Aug 2016
// RFB (VNC) implementation in golang
package gorfb

import (
	"bytes"
	"crypto/des"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
)

const (
	PROTOCOL  = "RFB 003.008\n"
	AUTH_FAIL = "Authentication Failure"
)

// PixelFormat information as required by protocol
type PixelFormat struct {
	BitsPerPixel uint8
	Depth        uint8  // Normally same as BitsPerPixel
	BigEndian    uint8  // 1 for BigEndian
	TrueColor    uint8  // 1 If the pixels are sent through as actual colors then following entries are used
	RedMax       uint16 // Maximum value that red can be 0->RedMax
	GreenMax     uint16
	BlueMax      uint16
	RedShift     uint8 // When you take the pixel value sent through how many bits must be shift to get the red
	GreenShift   uint8
	BlueShift    uint8
}

// RFBServer is the basic information that we need to start a RFB
type RFBServer struct {
	// On which port do we start the server (5900 is used as default)
	Port string
	// Pixel Width of the FrameBuffer
	Width int
	// Pixel Height of the FrameBuffer
	Height      int
	PixelFormat PixelFormat
	BufferName  string
	// The handler that will handle client requests
	Handler RFBServerHandler
	// Is authentication to be use
	Authenticate bool
	// If authentication is to be used, AuthText is the string to authenticate against
	AuthText string
}

// RFBConn is created when a successful TCP/IP connection was made with the client
type RFBConn struct {
	// Link to the server info that was used to create this connection
	Server *RFBServer
	// The Socket connection to the client
	Conn net.Conn
}

// RFBServerHandler is an interface with the function to handle requests
type RFBServerHandler interface {
	// Init is called as soon as RFB has been successfully established with client. Used by app to initialize any info for a session
	// conn is the RFB connection with the client, it is used by an app to send image data as well as cuttext information
	Init(conn *RFBConn)
	// Handle any requests from client for a specific pixel format
	// conn is the RFB connection with the client
	// pf is the PixelFormat information requested by the client
	ProcessSetPixelFormat(conn *RFBConn, pf PixelFormat)
	// Handle indication by client what encoding formats can be used (for now we ignore them and use raw)
	// conn is the RFB connection with the client
	// encodings is a slice of encodings supported by the client (refer to protocol)
	ProcessSetEncoding(conn *RFBConn, encodings []int)
	// Handle request by client to send part of
	// conn is the RFB connection with the client
	// x,y,width,height is the bounds of the rectangle that need to be send back
	// incremental indicates if it is to be an incremental update or a full update
	ProcessUpdateRequest(conn *RFBConn, x, y, width, height int, incremental bool)
	// Handle Keys send by client
	// conn is the RFB connection with the client
	// key is the key that was pressed
	// downflag indicated if the key is currently pressed down or released
	ProcessKeyEvent(conn *RFBConn, key int, downflag bool)
	// Handle any Pointer events send by client (normally mouse events)
	// conn is the RFB connection with the client
	// x, y are the coordinates of the pointer
	// button is a mask of which buttons were used (it is a bitmask)
	ProcessPointerEvent(conn *RFBConn, x, y, button int)
	// Handle any text send by the client (normally pasted text)
	// conn is the RFB connection with the client
	// text is the actual text sent
	ProcessCutText(conn *RFBConn, text string)
}

type RFBRectangle struct {
	X, Y, Width, Height int
	Buffer              []byte
}

// agreeProtocol is used to first agree on RFB3.8 as the protocol to use
// if an error is experienced at any point false is returned
func (fb *RFBConn) agreeProtocol() bool {
	sndsz, err := fmt.Fprintf(fb.Conn, PROTOCOL)
	if err != nil {
		log.Printf("Error sending server protocol: %s\n", err.Error())
		return false
	}
	if sndsz != len(PROTOCOL) {
		log.Println("Full protocol version was not sent to client!")
		return false
	}
	buf := make([]byte, 12)
	sz, err := fb.Conn.Read(buf)
	if err != nil {
		log.Printf("Error receiving client protocol: %s\n", err.Error())
		return false
	}
	if string(buf[:sz]) != PROTOCOL {
		log.Println("The client doesn't support RFB3.8!")
		return false
	}
	return true

}

// fixDesKeyByte is used to mirror a byte's bits
// This is not clearly indicated by the document, but is in actual fact used
func fixDesKeyByte(val byte) byte {
	var newval byte = 0
	for i := 0; i < 8; i++ {
		newval <<= 1
		newval += (val & 1)
		val >>= 1
	}
	return newval
}

// fixDesKey will make sure that exactly 8 bytes is used either by truncating or padding with nulls
// The bytes are then bit mirrored and returned
func fixDesKey(key string) []byte {
	tmp := []byte(key)
	buf := make([]byte, 8)
	if len(tmp) <= 8 {
		copy(buf, tmp)
	} else {
		copy(buf, tmp[:8])
	}
	for i := 0; i < 8; i++ {
		buf[i] = fixDesKeyByte(buf[i])
	}
	return buf
}

// agreeSecurity does the agreement on the security between server and client
// Currently only no auth is used, it will be changed shortly
func (fb *RFBConn) agreeSecurity() bool {
	buf := make([]byte, 8+len([]byte(AUTH_FAIL)))
	buf[0] = 1
	if fb.Server.Authenticate {
		buf[1] = 2 // Client must authenticate
	} else {
		buf[1] = 1 // No authentication
	}
	sndsz, err := fb.Conn.Write(buf[:2])
	if sndsz != 2 || err != nil {
		log.Printf("Error sending security types: %s\n", err.Error())
		return false
	}
	sz, err := fb.Conn.Read(buf[:1])
	if sz != 1 || err != nil {
		log.Printf("Error reading security type from client: %s\n", err.Error())
		return false
	}
	log.Printf("Security type %d requested by client\n", buf[0])
	if fb.Server.Authenticate {
		rand.Read(buf[:16]) // Random 16 bytes in buf
		sndsz, err = fb.Conn.Write(buf[:16])
		if err != nil {
			log.Printf("Error sending challenge to client: %s\n", err.Error())
			return false
		}
		if sndsz != 16 {
			log.Printf("The full 16 byte challenge was not sent!\n")
			return false
		}
		buf2 := make([]byte, 16)
		_, err := fb.Conn.Read(buf2)
		if err != nil {
			log.Printf("The authentication result was not read: %s\n", err.Error())
			return false
		}
		bk, err := des.NewCipher([]byte(fixDesKey(fb.Server.AuthText)))
		if err != nil {
			log.Printf("Error generating authentication cipher: %s\n", err.Error())
			return false
		}
		buf3 := make([]byte, 16)
		bk.Encrypt(buf3, buf)               //Encrypt first 8 bytes
		bk.Encrypt(buf3[8:], buf[8:])       // Encrypt second 8 bytes
		if bytes.Compare(buf2, buf3) != 0 { // If the result does not decrypt correctly to what we sent then a problem
			SetUint32(buf, 0, 1)
			SetUint32(buf, 4, uint32(len([]byte(AUTH_FAIL))))
			copy(buf[8:], []byte(AUTH_FAIL))
			fb.Conn.Write(buf)
			return false
		}
	}
	// Authentication was either none or it was successful
	SetUint32(buf, 0, 0)
	sndsz, err = fb.Conn.Write(buf[:4])
	if sndsz != 4 || err != nil {
		log.Printf("Error sending security successful notification: %s\n", err.Error())
		return false
	}
	log.Printf("Security successful notification sent!\n")
	return true

}

// performInit sends the dimensions and pixel information as part of the initializing phase
// If an error is experienced at any time a false is returned
func (fb *RFBConn) performInit() bool {
	buf := make([]byte, 100)
	_, err := fb.Conn.Read(buf[:1])
	if err != nil {
		log.Printf("Error reading init request from client: %s\n", err.Error())
		return false
	}
	log.Printf("Share buffer with other clients: %v\n", buf[0] == 1)
	SetUint16(buf, 0, uint16(fb.Server.Width))         // Buffer width
	SetUint16(buf, 2, uint16(fb.Server.Height))        // Buffer height
	buf[4] = fb.Server.PixelFormat.BitsPerPixel        // Bits per pixel
	buf[5] = fb.Server.PixelFormat.Depth               // Depth
	buf[6] = fb.Server.PixelFormat.BigEndian           // Big Endian
	buf[7] = fb.Server.PixelFormat.TrueColor           // True Color
	SetUint16(buf, 8, fb.Server.PixelFormat.RedMax)    // Max red
	SetUint16(buf, 10, fb.Server.PixelFormat.GreenMax) // Max green
	SetUint16(buf, 12, fb.Server.PixelFormat.BlueMax)  // Max blue
	buf[14] = fb.Server.PixelFormat.RedShift           // red shift
	buf[15] = fb.Server.PixelFormat.GreenShift         // green shift
	buf[16] = fb.Server.PixelFormat.BlueShift          // blue shift
	buf[17] = 0                                        // padding
	buf[18] = 0                                        // padding
	buf[19] = 0                                        // padding
	SetUint32(buf, 20, uint32(len(fb.Server.BufferName)))
	copy(buf[24:], []byte(fb.Server.BufferName))
	sz, err := fb.Conn.Write(buf[:24+len(fb.Server.BufferName)])
	if err != nil {
		log.Printf("Error sending init info: %s\n", err.Error())
		return false
	}
	if sz != 24+len(fb.Server.BufferName) {
		log.Printf("The init data was not sent to the client\n")
		return false
	}
	return true
}

// processClientRequest is the main loop to handle all incoming requests by the client
// for each request the appropriate call to the correct RFBServerHandler function is made
func (fb *RFBConn) processClientRequest() {
	defer fb.Conn.Close()
	for {
		buf := make([]byte, 100)
		_, err := fb.Conn.Read(buf[:1]) // Read the command byte sent by the client
		if err == nil {
			switch buf[0] {
			case 0: // Set Pixel Format
				_, err := fb.Conn.Read(buf[:19]) // Read the 16 bytes for the pixel format + 3 lead padding bytes
				if err != nil {
					log.Printf("Error reading info: %s\n", err.Error())
					return
				}
				pf := PixelFormat{buf[3], buf[4], buf[5], buf[6], GetUint16(buf, 7), GetUint16(buf, 9), GetUint16(buf, 11), buf[13], buf[14], buf[15]}
				fb.Server.Handler.ProcessSetPixelFormat(fb, pf)
			case 1: // FixColorMapEntries - not part of RFB 3.8 but some VNC clients send it anyway. We just ignore it
				_, err := fb.Conn.Read(buf[:6])
				if err != nil {
					log.Printf("Error reading FixColorMapEntries (1): %s\n", err.Error())
					return
				}
				cnt := int(GetUint16(buf, 4))
				tmpbuf := make([]byte, 6*cnt)
				_, err = fb.Conn.Read(tmpbuf)
				if err != nil {
					log.Printf("Error reading FixColorMapEntries (2): %s\n", err.Error())
					return
				}
			case 2: // Set Encoding
				_, err := fb.Conn.Read(buf[:3]) // Read 3 bytes with encoding count (number of encodings following)
				if err != nil {
					log.Printf("Error reading count of encoding types: %s\n", err.Error())
					return
				}
				cnt := int(GetUint16(buf, 1))      // Get count from buffer
				_, err = fb.Conn.Read(buf[:cnt*4]) // For the number of encodings times 4 (for uint32) read the encodings
				if err != nil {
					log.Printf("Error reading encoding types: %s\n", err.Error())
					return
				}
				encodings := make([]int, cnt)
				for i := 0; i < cnt; i++ {
					encodings[i] = int(GetUint32(buf, i*4))
				}
				fb.Server.Handler.ProcessSetEncoding(fb, encodings)
			case 3: // FB Update Request
				_, err := fb.Conn.Read(buf[:9]) // Read the bounds of the rectangle requested as well as the incremental flag
				if err != nil {
					log.Printf("Error reading Frame Buffer Update info: %s\n", err.Error())
					return
				}
				inc := buf[0]
				x := int(GetUint16(buf, 1))
				y := int(GetUint16(buf, 3))
				width := int(GetUint16(buf, 5))
				height := int(GetUint16(buf, 7))
				fb.Server.Handler.ProcessUpdateRequest(fb, x, y, width, height, inc == 1)
			case 4: // Key Event
				_, err := fb.Conn.Read(buf[:7]) // Read the key and the downflag
				if err != nil {
					fmt.Printf("Error reading Key RFBEvent info: %s\n", err.Error())
					return
				}
				downflag := buf[0] == 1
				key := int(GetUint32(buf, 3))
				fb.Server.Handler.ProcessKeyEvent(fb, key, downflag)
			case 5: // Pointer Event
				_, err := fb.Conn.Read(buf[:5]) // Read the coordinates and the button mask
				if err != nil {
					log.Printf("Error reading Pointer RFBEvent info: %s\n", err.Error())
					return
				}
				buttonmask := int(buf[0])
				x := int(GetUint16(buf, 1))
				y := int(GetUint16(buf, 3))
				fb.Server.Handler.ProcessPointerEvent(fb, x, y, buttonmask)
			case 6: // Client Cut Text - normally text pasted by the client
				_, err := fb.Conn.Read(buf[:7]) // Read the length of the text that was send
				if err != nil {
					log.Printf("Error reading Client Cut Text info: %s\n", err.Error())
					return
				}
				sz := int(GetUint32(buf, 3)) // Get the text length from the buffer
				buf2 := make([]byte, sz)     // Read the actual text
				_, err = fb.Conn.Read(buf2)
				if err != nil {
					log.Printf("Error reading client cut text: %s\n", err.Error())
					return
				}
				cuttext := string(buf2)
				fb.Server.Handler.ProcessCutText(fb, cuttext)
			default:
				log.Printf("Unknown cmd received (%d)\n", buf[0])
			}
		} else {
			if err != nil {
				log.Printf("Error: %s\n", err.Error())
				return
			} else {
				log.Printf("Nothing to read!\n")
			}
		}
	}
}

// process will do the initial handshaking and initialize a RFB connection with the client and then process any client requests
// Once the handshaking and initializing has been done the Init function of the handler is called to initialize whatever the server app needs
// Then the client requests are processed as they come in
func (fb *RFBConn) process() {
	if fb.agreeProtocol() && fb.agreeSecurity() && fb.performInit() {
		fb.Server.Handler.Init(fb)
		fb.processClientRequest()
	}
	fb.Conn.Close()
}

// SendCutText will send text back to client (normally copied text)
// text is the text that need to be send to the client
func (fb *RFBConn) SendCutText(text string) error {
	buf := make([]byte, 8+len([]byte(text)))     // Make byte buffer for command byte, length and actual string
	buf[0] = 3                                   // Command byte
	SetUint32(buf, 4, uint32(len([]byte(text)))) // Length of text
	copy(buf[8:], []byte(text))                  // Text to be sent
	_, err := fb.Conn.Write(buf)                 //Send it
	if err != nil {
		return err
	}
	return nil
}

// SendRectangle sends a rectangle of image information to the client
// x,y,width,height is the bounds of the rectangle
// buf is the actual image data that is in the format indicated by the PixelFormat
func (fb *RFBConn) SendRectangles(rects []RFBRectangle) error { //x, y, width, height int, buf []byte) error {
	tmpbuf := make([]byte, 4)
	tmpbuf[0] = 0                            // Command byte
	SetUint16(tmpbuf, 2, uint16(len(rects))) // Number of rectangles
	_, err := fb.Conn.Write(tmpbuf)
	if err != nil {
		return err
	}
	for _, rect := range rects {
		tmpbuf = make([]byte, 12+len(rect.Buffer))
		SetUint16(tmpbuf, 0, uint16(rect.X))
		SetUint16(tmpbuf, 2, uint16(rect.Y))
		SetUint16(tmpbuf, 4, uint16(rect.Width))
		SetUint16(tmpbuf, 6, uint16(rect.Height))
		SetUint32(tmpbuf, 8, uint32(0)) // Encoding = Raw. Will change as other encodings are implemented
		copy(tmpbuf[12:], rect.Buffer)
		_, err := fb.Conn.Write(tmpbuf)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartServer will start a server waiting for connections on the port as specified by the RFBServer port
// If Port is missing use the default of 5900
// For each connection handshaking is done and initialization and then client requests are handled and send to the Handler
func (rfb *RFBServer) StartServer() error {
	if rfb.Port == "" {
		rfb.Port = "5900"
	}
	if rfb.Authenticate && len(rfb.AuthText) == 0 {
		return errors.New("For authentication a authentication string must be provided!")
	}
	if rfb.Width <= 0 || rfb.Height <= 0 {
		return errors.New("Width and Height must be provided in RFBServer and they must be positive values!")
	}
	if rfb.Handler == nil {
		return errors.New("A handler must be provided!")
	}
	if rfb.PixelFormat.BitsPerPixel != 8 && rfb.PixelFormat.BitsPerPixel != 16 && rfb.PixelFormat.BitsPerPixel != 24 && rfb.PixelFormat.BitsPerPixel != 32 {
		return errors.New("Only 8, 16, 24 and 32 bits per pixel allowed")
	}
	if rfb.PixelFormat.TrueColor == 1 {
		if rfb.PixelFormat.RedMax == 0 || rfb.PixelFormat.GreenMax == 0 || rfb.PixelFormat.BlueMax == 0 {
			return errors.New("Provide maximum values for red, green and blue in the PixelFormat structure")
		}
		if rfb.PixelFormat.RedShift == rfb.PixelFormat.GreenShift || rfb.PixelFormat.RedShift == rfb.PixelFormat.BlueShift || rfb.PixelFormat.GreenShift == rfb.PixelFormat.BlueShift {
			return errors.New("None of the shifts can be the same!")
		}
	}
	ln, err := net.Listen("tcp", ":"+rfb.Port)
	if err != nil {
		return errors.New(fmt.Sprintf("Error listening on port %s: %s", rfb.Port, err.Error()))
	}
	for {
		con, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting incoming connection: %s\n", err.Error())
		} else {
			rfbcon := &RFBConn{Server: rfb, Conn: con}
			go rfbcon.process()
		}
	}
	return nil
}
