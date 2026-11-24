package money

import (
	"errors"
	"regexp"
	"strings"
)

var chatCurrencyValue = regexp.MustCompile(`^[+-]?[0-9.]`)

func chatWord(word string) string {
	return strings.Trim(strings.ToLower(word), ".,;:!?()[]{}\"'")
}

func chatPhrase(words []string, start int, phrase string) bool {
	parts := strings.Fields(phrase)
	if start+len(parts) > len(words) {
		return false
	}
	for index, part := range parts {
		if chatWord(words[start+index]) != part {
			return false
		}
	}
	return true
}

func remainingChatWords(words []string, removed []bool) []string {
	result := make([]string, 0, len(words))
	for index, word := range words {
		if !removed[index] {
			result = append(result, word)
		}
	}
	return result
}

func chatType(words []string) (string, []string) {
	if len(words) == 0 {
		return "", words
	}
	kinds := map[string]string{
		"expense": "expense", "spent": "expense", "paid": "expense",
		"income": "income", "received": "income",
		"transfer": "transfer", "transferred": "transfer",
	}
	index := 0
	if chatWord(words[0]) == "i" && len(words) > 1 {
		index = 1
	}
	if kind := kinds[chatWord(words[index])]; kind != "" {
		return kind, words[index+1:]
	}
	return "", words
}

// Accept an RM/MYR amount anywhere, or a bare amount at the start. Descriptive
// numbers such as "lunch for 2" stay in the note. Two currency amounts are
// rejected so the bot cannot silently save just part of a message.
func chatAmount(words []string) (int64, []string, error) {
	removed := make([]bool, len(words))
	amountText := ""
	for index := 0; index < len(words); index++ {
		word := chatWord(words[index])
		currency := ""
		value := ""
		for _, prefix := range []string{"myr", "rm"} {
			if strings.HasPrefix(word, prefix) {
				currency, value = prefix, strings.TrimPrefix(word, prefix)
				break
			}
			if strings.HasSuffix(word, prefix) {
				currency, value = prefix, strings.TrimSuffix(word, prefix)
				break
			}
		}
		if currency == "" || (value != "" && !chatCurrencyValue.MatchString(value)) {
			continue
		}
		if amountText != "" {
			return 0, nil, errors.New("send one amount per message, for example RM12.50 lunch")
		}
		removed[index] = true
		if value == "" {
			if index > 0 && !removed[index-1] && editAmountPattern.MatchString(chatWord(words[index-1])) {
				value = chatWord(words[index-1])
				removed[index-1] = true
			} else if index+1 < len(words) {
				index++
				value = chatWord(words[index])
				removed[index] = true
			}
		}
		if !editAmountPattern.MatchString(value) || strings.HasPrefix(value, "rm") {
			return 0, nil, errors.New("enter an RM amount with up to two decimal places, such as RM12.50")
		}
		amountText = value
	}
	if amountText != "" && len(words) > 0 && !removed[0] && editAmountPattern.MatchString(chatWord(words[0])) {
		return 0, nil, errors.New("send one amount per message, for example RM12.50 lunch")
	}
	if amountText == "" && len(words) > 0 {
		value := chatWord(words[0])
		if editAmountPattern.MatchString(value) {
			amountText, removed[0] = value, true
		}
	}
	if amountText == "" {
		return 0, nil, errors.New("include an amount, for example RM12.50 lunch today tng")
	}
	amount, err := chatAmountSen(amountText)
	return amount, remainingChatWords(words, removed), err
}

func takeChatCategory(kind string, words []string) (string, []string) {
	for _, category := range chatCategories[kind] {
		phrase := strings.ToLower(category)
		if chatPhrase(words, 0, phrase) {
			return category, words[len(strings.Fields(phrase)):]
		}
	}
	return "", words
}

type categoryKeywords struct {
	category string
	phrases  []string
}

var chatCategoryKeywords = map[string][]categoryKeywords{
	"expense": {
		{"Food", []string{"food", "breakfast", "brunch", "lunch", "dinner", "supper", "meal", "meals", "coffee", "tea", "snack", "snacks", "groceries", "grocery", "grabfood", "foodpanda"}},
		{"Transport", []string{"transport", "grab", "taxi", "bus", "train", "mrt", "lrt", "ktm", "petrol", "fuel", "diesel", "parking", "toll", "flight", "airfare"}},
		{"Bills", []string{"bill", "bills", "rent", "electricity", "water bill", "internet", "wifi", "utilities", "phone bill"}},
		{"Health", []string{"health", "doctor", "clinic", "medicine", "pharmacy", "dentist", "dental", "hospital"}},
		{"Shopping", []string{"shopping", "shopee", "lazada", "clothes", "clothing", "shoes", "electronics", "stationery"}},
		{"Family", []string{"family", "school", "tuition", "childcare", "parents"}},
		{"Entertainment", []string{"entertainment", "movie", "movies", "cinema", "netflix", "spotify", "game", "games", "concert"}},
	},
	"income": {
		{"Salary", []string{"salary", "paycheck", "wages", "bonus"}},
		{"Freelance", []string{"freelance", "freelancing", "client payment"}},
		{"Refund", []string{"refund", "cashback", "reimbursement"}},
		{"Gift", []string{"gift", "present"}},
		{"Investment", []string{"investment", "dividend", "dividends", "interest"}},
	},
	"transfer": {
		{"Top Up", []string{"top up", "topup", "reload"}},
		{"Bank Move", []string{"bank move", "bank transfer"}},
		{"Card Payment", []string{"card payment", "credit card payment"}},
		{"Savings", []string{"savings", "save"}},
	},
}

func inferChatCategory(kind string, words []string) string {
	for _, rule := range chatCategoryKeywords[kind] {
		for _, phrase := range rule.phrases {
			for index := range words {
				if chatPhrase(words, index, phrase) {
					return rule.category
				}
			}
		}
	}
	return "Other"
}
