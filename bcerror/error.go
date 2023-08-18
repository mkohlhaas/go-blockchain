package bcerror

import "log"

// Prints error and panics.
func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}
