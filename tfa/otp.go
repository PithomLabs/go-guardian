package tfa

import (
	"fmt"
	"time"
)

// OTP represents one-time password algorithm for both HOTP and TOTP.
type OTP interface {
	// Interval returns sequential value to be used in HMAC.
	// If targted OTP is HOTP returns counter value,
	// Otherwise, return time-step (current UTC time / period)
	Interval() uint64
	// Secret return OTP shared secret.
	Secret() string
	// Algorithm return OTP hashing algorithm.
	Algorithm() HashAlgorithm
	// Digits return OTP digits.
	Digits() Digits
	// Verify increments its interval and then calculates the next OTP value.
	// If the received value matches the calculated value then the OTP value is valid.
	// Verify follow validation of OTP values as described in RFC 6238 section 4.1 and
	// And RFC 4226 section 7.2.
	// To enable throttling at the server and stop brute force attacks,
	// The Verify method lunch lockout mechanism based on a predefined configuration.
	// The lockout mechanism implement a delay scheme and failed OTP counter,
	// Each time OTP verification failed the delay scheme increased by delay*failed, number of seconds,
	// And client must wait for the delay window, Otherwise, an error returned  verification process disabled.
	// Once the max attempts reached the verification process return error indicate account has been blocked.
	// Lockout mechanism disabled by default, See OTPConfig to learn more about lockout configuration.
	// Lockout follow Throttling at the Server as described in RFC 4226 section 7.3 .
	Verify(otp string) (bool, error)
	// EnableLockout enable or disable lockout mechanism
	EnableLockout(e bool)
	// SetMaxAttempts of verification failures to lock the account.
	SetMaxAttempts(max uint)
	// SetDealy window to periodically disable password verification process.
	SetDealy(dealy uint)
	// SetStartAt, set in what attempt number, lockout mechanism start to work.
	SetStartAt(num uint)
	// Failed return count of failed verification.
	Failed() uint
	// SetFailed, set count of failed verification.
	SetFailed(num uint)
	// DelayTime return time represents the end of the disabling password verification process.
	DelayTime() time.Time
	// SetDelayTime set time that represents the end of the disabling password verification process.
	SetDelayTime(t time.Time)
}

type baseOTP struct {
	key           *Key
	enableLockout bool
	startAt       uint
	startAtB      uint
	dealy         uint
	maxAttempts   uint
	failed        uint
	dealyTime     time.Time
}

func (b *baseOTP) EnableLockout(e bool)     { b.enableLockout = e }
func (b *baseOTP) SetDelayTime(t time.Time) { b.dealyTime = t }
func (b *baseOTP) SetFailed(num uint)       { b.failed = num }
func (b *baseOTP) SetStartAt(num uint)      { b.startAt = num }
func (b *baseOTP) SetDealy(dealy uint)      { b.dealy = dealy }
func (b *baseOTP) SetMaxAttempts(max uint)  { b.maxAttempts = max }
func (b *baseOTP) DelayTime() time.Time     { return b.dealyTime }
func (b *baseOTP) Failed() uint             { return b.failed }
func (b *baseOTP) Secret() string           { return b.key.Secret() }
func (b *baseOTP) Digits() Digits           { return b.key.Digits() }
func (b *baseOTP) Algorithm() HashAlgorithm { return b.key.Algorithm() }
func (b *baseOTP) lockOut() error {

	if b.failed == 0 {
		return nil
	}

	if b.failed == b.maxAttempts {
		return fmt.Errorf("Max attempts reached, Account locked out")
	}

	if remaining := b.dealyTime.UTC().Sub(time.Now().UTC()); remaining > 0 {
		return fmt.Errorf("Password verification disabled, Try again in %s", remaining)
	}

	return nil
}

func (b *baseOTP) updateLockOut(valid bool) {
	if !b.enableLockout || valid {
		b.startAt = b.startAtB
		return
	}

	if b.startAt > 1 {
		b.startAt--
		return
	}

	b.failed++
	b.dealyTime = time.Now().UTC().Add(time.Second * time.Duration(b.failed*b.dealy))
}

type totp struct {
	*baseOTP
}

func (t *totp) Verify(otp string) (bool, error) {
	err := t.lockOut()
	if err != nil {
		return false, err
	}
	code, err := GenerateOTP(t)
	result := code == otp
	t.updateLockOut(result)
	return result, err
}

func (t *totp) Interval() uint64 {
	return uint64(time.Now().UTC().Unix()) / t.key.Period()
}

type hotp struct {
	*baseOTP
}

func (h *hotp) Verify(otp string) (bool, error) {
	err := h.lockOut()
	if err != nil {
		return false, err
	}
	code, err := GenerateOTP(h)
	result := code == otp
	h.updateLockOut(result)
	return result, err
}

func (h *hotp) Interval() uint64 {
	counter := h.key.Counter()
	counter++
	h.key.SetCounter(counter)
	return counter
}
