package bitwrk

import (
	"bitwrk/money"
	"fmt"
	"time"
)

type Account interface {
	GetBalance() money.Money
	CanApply(delta money.Money) bool
	Apply(delta money.Money) error
	GetLastMovementKey() *string
}

type ParticipantAccount struct {
	Participant        string
	LastMovementKey    *string
	Available, Blocked money.Money
}

func (a *ParticipantAccount) GetAvailable() Account {
	return participantAccountPart{a, &a.Available}
}

func (a *ParticipantAccount) GetBlocked() Account {
	return participantAccountPart{a, &a.Blocked}
}

type participantAccountPart struct {
	account *ParticipantAccount
	balance *money.Money
}

func (a participantAccountPart) GetBalance() money.Money {
	return *a.balance
}

func (a participantAccountPart) CanApply(delta money.Money) bool {
	return a.balance.Currency == delta.Currency && delta.Amount+a.balance.Amount >= 0
}

func (a participantAccountPart) Apply(delta money.Money) error {
	if !a.CanApply(delta) {
		return fmt.Errorf("Can't apply delta of %v to balance of %v", delta, a.balance)
	}
	a.balance.Amount += delta.Amount
	return nil
}

func (a participantAccountPart) GetLastMovementKey() *string {
	return a.account.LastMovementKey
}

type AccountMovement struct {
	Key       *string
	Timestamp time.Time
	Type      AccountMovementType

	AvailableDelta          money.Money
	AvailableAccount        string
	AvailablePredecessorKey *string

	BlockedDelta          money.Money
	BlockedAccount        string
	BlockedPredecessorKey *string

	Fee money.Money

	//BidKey, TransactionKey *string
}

// Places a new account movement between the given accounts, in a
// transaction-safe way. If amy error but occurs, the transaction must be rolled
// back.
func PlaceAccountMovement(
	dao AccountingDao,
	now time.Time,
	mType AccountMovementType,
	availableParticipant, blockedParticipant string,
	availableDelta, blockedDelta, fee money.Money,
) error {
	m := new(AccountMovement)
	m.Timestamp = now
	m.Type = mType
	m.AvailableDelta = availableDelta
	m.AvailableAccount = availableParticipant
	m.BlockedDelta = blockedDelta
	m.BlockedAccount = blockedParticipant
	m.Fee = fee

	if err := m.Validate(); err != nil {
		return err
	}

	// Fetch new key
	if key, err := dao.NewAccountMovementKey(availableParticipant); err != nil {
		return err
	} else {
		m.Key = &key
	}

	// Apply movement to available account
	if account, err := dao.GetAccount(availableParticipant); err != nil {
		return err
	} else {
		if err := account.GetAvailable().Apply(availableDelta); err != nil {
			return err
		}
		m.AvailablePredecessorKey = account.LastMovementKey
		account.LastMovementKey = m.Key

		if err := dao.SaveAccount(&account); err != nil {
			return err
		}
	}

	// Apply movement to blocked account
	if account, err := dao.GetAccount(blockedParticipant); err != nil {
		return err
	} else {
		if err := account.GetBlocked().Apply(blockedDelta); err != nil {
			return err
		}
		m.BlockedPredecessorKey = account.LastMovementKey
		account.LastMovementKey = m.Key
		if err := dao.SaveAccount(&account); err != nil {
			return err
		}
	}

	// Save movement
	if err := dao.SaveMovement(m); err != nil {
		return err
	}

	return nil
}

type AccountMovementType int8

const (
	AccountMovementInvalid AccountMovementType = iota
	AccountMovementPayIn
	AccountMovementPayout
	AccountMovementPayoutReimburse
	AccountMovementBid
	AccountMovementBidReimburse
	AccountMovementTransaction
	AccountMovementTransactionFinish
	AccountMovementTransactionReimburse
)

func (t AccountMovementType) String() string {
	switch t {
	case AccountMovementPayIn:
		return "PAYIN"
	case AccountMovementPayout:
		return "PAYOUT"
	case AccountMovementPayoutReimburse:
		return "PAYOUT_REIMBURSE"
	case AccountMovementBid:
		return "BID"
	case AccountMovementBidReimburse:
		return "BID_REIMBURSE"
	case AccountMovementTransaction:
		return "TRANSACTION"
	case AccountMovementTransactionFinish:
		return "TRANSACTION_FINISH"
	case AccountMovementTransactionReimburse:
		return "TRANSACTION_REIMBURSE"
	}
	return fmt.Sprintf("<Invalid Account Moment Type: %v>", int8(t))
}

func (m *AccountMovement) String() string {
	return fmt.Sprintf("%v: %v/UNBLOCKED:%v %v/BLOCKED:%v fee:%v",
		m.Type,
		m.AvailableAccount, m.AvailableDelta,
		m.BlockedAccount, m.BlockedDelta,
		m.Fee)
}

func validateCurrency(currency *money.Currency, otherCurrency money.Currency) (*money.Currency, error) {
	if currency == nil {
		return &otherCurrency, nil
	}
	if otherCurrency != *currency {
		return currency, fmt.Errorf("Currencies mixed: %v / %v", currency, otherCurrency)
	}
	return currency, nil
}

func checkFlowDirection(msg string, a int, b int64) error {
	if a < 0 && b < 0 || a >= -1 && a <= 1 && b == 0 || a > 0 && b > 0 {
		return nil
	} else {
		return fmt.Errorf("Wrong signedness in %v: %v contradicts %v", msg, a, b)
	}
	return nil // never reached
}

func (m *AccountMovement) checkCashFlowDirection(available, blocked, fee int) error {
	if err := checkFlowDirection("available", available, m.AvailableDelta.Amount); err != nil {
		return err
	}
	if err := checkFlowDirection("blocked", blocked, m.BlockedDelta.Amount); err != nil {
		return err
	}
	if err := checkFlowDirection("fee", fee, m.Fee.Amount); err != nil {
		return err
	}
	return nil
}

func (m *AccountMovement) Validate() (err error) {
	var currency *money.Currency

	var sum int64 = 0

	if m.AvailableDelta.Amount != 0 {
		currency, err = validateCurrency(currency, m.AvailableDelta.Currency)
		if err != nil {
			return
		}
		sum += m.AvailableDelta.Amount
	}

	if m.BlockedDelta.Amount != 0 {
		currency, err = validateCurrency(currency, m.BlockedDelta.Currency)
		if err != nil {
			return
		}
		sum += m.BlockedDelta.Amount
	}

	if m.Fee.Amount != 0 {
		currency, err = validateCurrency(currency, m.Fee.Currency)
		if err != nil {
			return
		}
		sum += m.Fee.Amount
	}

	switch m.Type {
	//checkCashFlowDirection(unblocked, blocked, fee int)
	case AccountMovementBid:
		err = m.checkCashFlowDirection(-2, 2, 0)
	case AccountMovementBidReimburse:
		err = m.checkCashFlowDirection(2, -2, 0)
	case AccountMovementTransaction:
		err = m.checkCashFlowDirection(1, -1, 0)
	case AccountMovementTransactionFinish:
		err = m.checkCashFlowDirection(2, -2, 1)
	case AccountMovementTransactionReimburse:
		err = m.checkCashFlowDirection(2, -2, 0)
	default:
		err = fmt.Errorf("Invalid account movement type %v", m.Type)
	}

	if err != nil {
		return
	}

	if sum != 0 {
		err = fmt.Errorf("Sum of amounts is not 0: %v", sum)
	}

	return
}

func (m *AccountMovement) MustValidate() {
	err := m.Validate()
	if err != nil {
		panic(fmt.Sprintf("%v doesn't validate: %v", m, err))
	}
}
