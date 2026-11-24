package money

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	chatISODate   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	chatLocalDate = regexp.MustCompile(`^([0-9]{1,2})/([0-9]{1,2})/([0-9]{4})$`)
	chatDateLike  = regexp.MustCompile(`^[0-9]+[-/][0-9]+[-/][0-9]+$`)
	chatDaysAgo   = regexp.MustCompile(`^[0-9]{1,3}$`)
)

func chatDate(words []string, sentAt time.Time) (string, []string, error) {
	location, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		return "", nil, errors.New("Malaysia time zone is unavailable")
	}
	sent := sentAt.In(location)
	chosen := sent.Format(time.DateOnly)
	priority := 0
	removed := make([]bool, len(words))
	for index := 0; index < len(words); index++ {
		word := chatWord(words[index])
		explicit := strings.HasPrefix(word, "@")
		value := strings.TrimPrefix(word, "@")
		date, found, err := chatDateValue(value, sent)
		if err != nil {
			return "", nil, err
		}
		count := 1
		if !found && chatPhrase(words, index, "last night") {
			date, found, count = sent.AddDate(0, 0, -1).Format(time.DateOnly), true, 2
		}
		if !found && index+2 < len(words) && chatDaysAgo.MatchString(value) &&
			(chatWord(words[index+1]) == "day" || chatWord(words[index+1]) == "days") &&
			chatWord(words[index+2]) == "ago" {
			days, _ := strconv.Atoi(value)
			date, found, count = sent.AddDate(0, 0, -days).Format(time.DateOnly), true, 3
		}
		if !found {
			if explicit {
				return "", nil, errors.New("use today, yesterday, or a date like 2026-11-24")
			}
			if (word == "last" || word == "next") && index+1 < len(words) {
				switch chatWord(words[index+1]) {
				case "week", "month", "year":
					return "", nil, errors.New("use an exact transaction date for that period, such as 2026-11-24")
				}
			}
			continue
		}
		level := 1
		if explicit {
			level = 2
		}
		if priority == level && chosen != date {
			return "", nil, errors.New("use one transaction date per message")
		}
		// Preserve the original @date format as an explicit override.
		if level >= priority {
			chosen, priority = date, level
		}
		if index > 0 {
			switch chatWord(words[index-1]) {
			case "on", "for", "date":
				removed[index-1] = true
			}
		}
		for offset := 0; offset < count; offset++ {
			removed[index+offset] = true
		}
		index += count - 1
	}
	return chosen, remainingChatWords(words, removed), nil
}

func chatDateValue(value string, sent time.Time) (string, bool, error) {
	offsets := map[string]int{
		"today": 0, "tdy": 0, "today's": 0,
		"yesterday": -1, "ytd": -1, "yesterday's": -1, "lastnight": -1,
		"tomorrow": 1, "tmr": 1, "tmrw": 1,
	}
	if offset, ok := offsets[value]; ok {
		return sent.AddDate(0, 0, offset).Format(time.DateOnly), true, nil
	}
	if parts := chatLocalDate.FindStringSubmatch(value); parts != nil {
		day, _ := strconv.Atoi(parts[1])
		month, _ := strconv.Atoi(parts[2])
		value = fmt.Sprintf("%s-%02d-%02d", parts[3], month, day)
	}
	if !chatISODate.MatchString(value) {
		if chatDateLike.MatchString(value) {
			return "", true, errors.New("enter a valid date as YYYY-MM-DD or DD/MM/YYYY")
		}
		return "", false, nil
	}
	date, err := time.Parse(time.DateOnly, value)
	if err != nil || strings.HasPrefix(value, "0000-") {
		return "", true, errors.New("enter a valid date as YYYY-MM-DD or DD/MM/YYYY")
	}
	return date.Format(time.DateOnly), true, nil
}

type accountAliases struct {
	name    string
	phrases []string
}

// Longer phrases precede their short forms so the complete account name is
// removed from the note. These names match the Money form.
var chatAccounts = []accountAliases{
	{"Touch 'n Go", []string{"touch n go ewallet", "touch and go", "touch n go", "tng ewallet", "touchngo", "touch'n'go", "tng", "t&g"}},
	{"Apple Pay", []string{"apple pay", "applepay"}},
	{"GrabPay", []string{"grab pay", "grabpay"}},
	{"Credit Card", []string{"credit card", "creditcard", "cc"}},
	{"Debit Card", []string{"debit card", "debitcard"}},
	{"Bank", []string{"bank"}},
	{"Cash", []string{"cash", "tunai"}},
}

func chatAccount(words []string) (string, []string, error) {
	account := "Unspecified"
	removed := make([]bool, len(words))
	for index := 0; index < len(words); index++ {
		matched := 0
		for _, aliases := range chatAccounts {
			for _, phrase := range aliases.phrases {
				if !chatPhrase(words, index, phrase) {
					continue
				}
				if account != "Unspecified" && account != aliases.name {
					return "", nil, errors.New("use one account per message, such as tng or cash")
				}
				account, matched = aliases.name, len(strings.Fields(phrase))
				break
			}
			if matched > 0 {
				break
			}
		}
		if matched == 0 {
			continue
		}
		if index > 0 {
			switch chatWord(words[index-1]) {
			case "using", "via", "with", "by", "from", "account":
				removed[index-1] = true
				if index > 1 && chatWord(words[index-2]) == "paid" {
					removed[index-2] = true
				}
			}
		}
		for offset := 0; offset < matched; offset++ {
			removed[index+offset] = true
		}
		index += matched - 1
	}
	return account, remainingChatWords(words, removed), nil
}
