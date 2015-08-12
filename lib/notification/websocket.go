package notification

import (
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// NewHub returns a new hub which is accepting notifications and connections.
// The hub stops accepting new requests when "ctx" is canceled.
func NewHub(ctx context.Context) *Hub {
	h := &Hub{
		broadcast:   make(chan string),
		register:    make(chan *connection),
		connections: make(map[*connection]context.CancelFunc),
	}
	go h.run(ctx)
	return h
}

// Hub is a hub of websocket connections.
// Those connections are connected to each other via this hub.
type Hub struct {
	// connections are the registered connections.
	connections map[*connection]context.CancelFunc

	// broadcast accepts inbound messages from the connections.
	broadcast chan string

	// register accepts new connections to be registered
	register chan *connection
}

// AcceptConnection receives a websocket connection and register it as a subscriber of broadcast notifications.
func (h *Hub) AcceptConnection(ws *websocket.Conn) {
	r, w := make(chan string, 256), h.broadcast
	c := connection{ws: ws, r: r, w: w, closed: make(chan struct{})}
	h.register <- &c
	<-c.closed
}

// Broadcast sends "msg" to the registered connections.
func (h *Hub) Broadcast(msg string) {
	h.broadcast <- msg
}

func (h *Hub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			h.connections[c] = c.start(ctx)
		case m := <-h.broadcast:
			for c, cancel := range h.connections {
				select {
				case <-c.closed:
					delete(h.connections, c)
				case c.r <- m:
				default:
					delete(h.connections, c)
					cancel()
				}
			}
		}
	}
}

// connection is a bidirectional pipe between a websocket connection and go channels.
type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	r chan string
	w chan<- string

	// closed is a channel which is closed when this connection is being closed
	closed chan struct{}
}

// start starts forwarding between internal channels and the websocket connection
// The forwarding loop stops when "ctx" is canceled or the function which this function returns is called.
func (c *connection) start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		c.writerLoop(ctx)
	}()
	go func() {
		defer cancel()
		c.readerLoop(ctx)
	}()
	go func() {
		<-ctx.Done()
		close(c.closed)
	}()
	return cancel
}

func (c *connection) writerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-c.r:
			err := websocket.Message.Send(c.ws, message)
			if err != nil {
				// TODO(yugui) add logging
				break
			}
		}
	}
}

func (c *connection) readerLoop(ctx context.Context) {
	defer c.ws.Close()
	ch := make(chan string, 1)
	for {
		go func() {
			var message string
			err := websocket.Message.Receive(c.ws, &message)
			if err != nil {
				// TODO(yugui) add logging
				close(ch)
				return
			}
			ch <- message
		}()
		select {
		case <-ctx.Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				return
			case c.w <- m:
			}
		}
	}
}
