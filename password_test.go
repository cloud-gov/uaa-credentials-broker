package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("password", func() {
	var (
		badPassword  password
		goodPassword password
	)

	BeforeEach(func() {
		badPassword = password{Password: "-badpassword", Upper: 0, Lower: 11, Number: 0, Special: 1}
		goodPassword = password{Password: "goodpassword", Upper: 0, Lower: 12, Number: 0, Special: 0}
	})

	Describe("Password generation", func() {
		Context("With a leading dash", func() {
			It("should generate a new password", func() {
				Expect(ValidatePassword(badPassword)).To(MatchError("Invalid password"))
			})
		})
		Context("Without a leading dash", func() {
			It("should accept the password", func() {
				Expect(ValidatePassword(goodPassword)).To(BeNil())
			})
		})
	})
})
