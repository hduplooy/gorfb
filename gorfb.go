// gorfb project gorfb.go
package gorfb

import (
	"fmt"
	"log"
	"net"
)

const (
	PROTOCOL = "RFB 003.008\n"
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
	Port        string           // On which port do we start the server (5900 is used as default)
	Width       int              // Pixel Width of the FrameBuffer
	Height      int              // Pixel Height of the FrameBuffer
	PixelFormat PixelFormat      // The pixel format used to represent pixel colors
	BufferName  string           // A name describing the buffer
	Handler     RFBServerHandler // The handler that will handle client requests
}

// RFBConn is created when a successful TCP/IP connection was made with the client
type RFBConn struct {
	Server *RFBServer // Link to the server info that was used to create this connection
	Conn   net.Conn   // The Socket connection to the client
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

// agreeProtocol is used to first agree on RFB3.8 as the protocol to use
// if an error is experienced at any point false is returned
func (fb *RFBConn) agreeProtocol() bool {
	sndsz, err := fmt.Fprintf(fb.Conn, PROTOCOL)
	if err != nil {
		log.Printf("Error sending server protocol: %s\n", err.Error())
		return false
	} else if sndsz != len(PROTOCOL) {
		log.Println("Full protocol version was not sent to client!")
		return false
	} else {
		buf := make([]byte, 20)
		sz, err := fb.Conn.Read(buf)
		if err != nil {
			log.Printf("Error receiving client protocol: %s\n", err.Error())
			return false
		} else if string(buf[:sz]) != PROTOCOL {
			log.Println("The client doesn't support RFB3.8!")
			return false
		} else {
			return true
		}
	}
}

// agreeSecurity does the agreement on the security between server and client
// Currently only no auth is used, it will be changed shortly
func (fb *RFBConn) agreeSecurity() bool {
	buf := make([]byte, 4)
	SetUint16(buf, 0, uint16(0x0101))
	sndsz, err := fb.Conn.Write(buf[:2])
	if sndsz != 2 || err != nil {
		log.Printf("Error sending security types: %s\n", err.Error())
		return false
	} else {
		log.Printf("Security types sent\n")
		sz, err := fb.Conn.Read(buf[:1])
		if sz != 1 || err != nil {
			log.Printf("Error reading security type from client: %s\n", err.Error())
			return false
		} else {
			log.Printf("Security type received from client: %d\n", buf[0])
			SetUint32(buf, 0, 0)
			sndsz, err = fb.Conn.Write(buf[:4])
			if sndsz != 4 || err != nil {
				log.Printf("Error sending security successful notification: %s\n", err.Error())
				return false
			} else {
				log.Printf("Security successful notification sent!\n")
				return true
			}
		}
	}

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
				_, err := fb.Conn.Read(buf[:16]) // Read the 16 bytes for the pixel format
				if err != nil {
					log.Printf("Error reading info: %s\n", err.Error())
					return
				}
				pf := PixelFormat{buf[0], buf[1], buf[2], buf[3], GetUint16(buf, 4), GetUint16(buf, 6), GetUint16(buf, 8), buf[10], buf[11], buf[12]}
				fb.Server.Handler.ProcessSetPixelFormat(fb, pf)

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
func (fb *RFBConn) SendRectangle(x, y, width, height int, buf []byte) error {
	tmpbuf := make([]byte, 16+len(buf))
	buf[0] = 0              // Command byte
	SetUint16(tmpbuf, 2, 1) // Number of rectangles
	SetUint16(tmpbuf, 4, uint16(x))
	SetUint16(tmpbuf, 6, uint16(y))
	SetUint16(tmpbuf, 8, uint16(width))
	SetUint16(tmpbuf, 10, uint16(height))
	SetUint32(tmpbuf, 12, uint32(0)) // Encoding = Raw. Will change as other encodings are implemented
	copy(tmpbuf[16:], buf)
	_, err := fb.Conn.Write(tmpbuf)
	if err != nil {
		return err
	}
	return nil
}

// StartServer will start a server waiting for connections on the port as specified by the RFBServer port
// If Port is missing use the default of 5900
// For each connection handshaking is done and initialization and then client requests are handled and send to the Handler
func (rfb *RFBServer) StartServer() {
	if rfb.Port == "" {
		rfb.Port = "5900"
	}
	ln, err := net.Listen("tcp", ":"+rfb.Port)
	if err != nil {
		log.Printf("Error listening on port %s: %s\n", rfb.Port, err.Error())
		return
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
}
