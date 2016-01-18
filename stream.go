package gitter
import (
	"bufio"
	"encoding/json"
	"net/http"
	"time"
)

var DEFAULT_CONNECTION_WAIT_TIME time.Duration = 3000 // millis
var DEFAULT_CONNECTION_MAX_RETRIES int = 5

// Initialize stream
func (gitter *Gitter) Stream(roomId string) *Stream {
	return &Stream{
		url: GITTER_STREAM_API + "rooms/" + roomId + "/chatMessages",
		GitterMessage: make(chan Message),
		gitter: gitter,
		streamConnection: gitter.newStreamConnection(
			DEFAULT_CONNECTION_WAIT_TIME,
			DEFAULT_CONNECTION_MAX_RETRIES),
	}
}

func (gitter *Gitter) Listen(stream *Stream) {

	var reader *bufio.Reader
	var gitterMessage Message

	// connect
	stream.connect()

	Loop:
	for {

		// if closed then stop trying
		if stream.isClosed() {
			break Loop
		}

		reader = bufio.NewReader(stream.getResonse().Body)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			gitter.log(err)
			stream.connect()
			continue
		}

		// unmarshal the streamed data
		err = json.Unmarshal(line, &gitterMessage)
		if err != nil {
			gitter.log(err)
			continue
		}

		// we are here, then we got the good message. pipe it forward.
		stream.GitterMessage <- gitterMessage
	}

	gitter.log("Listening was completed")
}

// Definition of stream
type Stream struct {
	url              string
	GitterMessage    chan Message
	streamConnection *streamConnection
	gitter           *Gitter
}

// connect and try to reconnect with
func (stream *Stream) connect() {

	if (stream.streamConnection.retries == stream.streamConnection.currentRetries) {
		stream.Close()
		stream.gitter.log("Number of retries exceeded the max retries number, we are done here")
		return
	}

	res, err := stream.gitter.getResponse(stream.url)
	if err != nil || res.StatusCode != 200 {
		stream.gitter.log("Failed to get response, trying reconnect")
		stream.gitter.log(err)

		// sleep and wait
		stream.streamConnection.currentRetries += 1
		time.Sleep(time.Millisecond * stream.streamConnection.wait * time.Duration(stream.streamConnection.currentRetries))

		// connect again
		stream.connect()

	} else {
		stream.gitter.log("Successfully connected")
		stream.streamConnection.currentRetries = 0
		stream.streamConnection.closed = false
		stream.streamConnection.response = res
	}
}

type streamConnection struct {

	// connection was closed
	closed         bool

	// wait time till next try
	wait           time.Duration

	// max tries to recover
	retries        int

	// current streamed response
	response       *http.Response

	// current status
	currentRetries int

}

// Close the stream connection and stop receiving streamed data
func (stream *Stream) Close() {
	conn := stream.streamConnection
	conn.closed = true
	if conn.response != nil {
		stream.gitter.log("Stream connection was closed")
		conn.response.Body.Close()
	}
	conn.currentRetries = 0
}

func (stream *Stream) isClosed() bool {
	return stream.streamConnection.closed
}

func (stream *Stream) getResonse() *http.Response {
	return stream.streamConnection.response
}

// Optional, set stream connection properties
// wait - time in milliseconds of waiting between reconnections. Will grow exponentially.
// retries - number of reconnections retries before dropping the stream.
func (gitter *Gitter) newStreamConnection(wait time.Duration, retries int) *streamConnection {
	return &streamConnection{
		closed: true,
		wait: wait,
		retries : retries,
	}
}