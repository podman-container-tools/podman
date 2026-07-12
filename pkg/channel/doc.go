/*
Package channel provides helper structs/methods/funcs for working with channels

Proxy from an io.Writer to a channel:

		w := channel.NewWriter(make(chan []byte, 10))
		go func() {
			w.Write([]byte("Hello, World"))
		}()

		fmt.Println(string(<-w.Chan()))
	    w.Close()

Use of the constructor is required to initialize the channel.
Provide a channel of sufficient size to handle messages from writer(s).
*/
package channel
