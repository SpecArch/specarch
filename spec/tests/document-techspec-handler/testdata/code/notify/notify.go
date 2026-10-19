package notify

import "net/http"

// Carrier takes messages out.
type Carrier interface {
	Send(message string) error
}

// carrier is the carrier in use.
var carrier Carrier

// Collector tells a parcel's collector that it waits.
func Collector(code string) {
	message := compose(code)
	req, _ := http.NewRequest(http.MethodPost, "https://messages.example.com/v1/send", nil)
	_ = req
	send(message)
}

// compose is the text of the message.
func compose(code string) string {
	return "Your parcel " + code + " waits in a locker."
}

// send hands the message to the carrier.
func send(message string) {
	deliver(message)
	carrier.Send(message)
}

// deliver is the carrier's own step.
func deliver(message string) {
	stamp(message)
}

// stamp dates the message.
func stamp(message string) {
	_ = message
}
