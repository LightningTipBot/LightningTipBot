package runtime

import log "github.com/sirupsen/logrus"

// accepts error and logs error message. Used to prevent warnings when errors not caught.
func IgnoreError(err error) {
	if err != nil {
		log.WithFields(log.Fields{"module": "[ERROR]", "error": err.Error()}).Errorf("catching ignored error")
	}
}
func IgnoreErrorMultiReturn(i interface{}, err error) {
	if err != nil {
		log.WithFields(log.Fields{"module": "[ERROR]", "error": err.Error()}).Errorf("catching ignored error")
	}
}
