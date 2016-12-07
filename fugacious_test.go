package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net/http"
	"net/http/httptest"
)

var _ = Describe("Fugacious", func() {
	var (
		header int
		server *httptest.Server
		sender FugaciousCredentialSender
	)

	BeforeEach(func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "https://fugacio.us/m/42")
			w.WriteHeader(header)
		}))
		sender = FugaciousCredentialSender{
			endpoint: server.URL,
			hours:    24,
			maxViews: 4,
		}
	})

	Context("when the status code is expected", func() {
		BeforeEach(func() {
			header = http.StatusCreated
		})

		It("retrieves a link from the fugacious server", func() {
			url, err := sender.Send("testing")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("https://fugacio.us/m/42"))
		})
	})

	Context("when the status code is unexpected", func() {
		BeforeEach(func() {
			header = http.StatusNotFound
		})

		It("complains about the response status", func() {
			_, err := sender.Send("testing")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Expected status"))
		})
	})
})
