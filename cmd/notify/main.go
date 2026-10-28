package main

import (
	"errors"
	"fmt"

	"lesson5/student/Go_X5Tech_21_october_practice/practice"
)

func main() {
	fmt.Println("=&= Успех =&=")
	email := &practice.EmailNotifier{
		BaseNotifier: practice.BaseNotifier{Channel: "email"},
		Address:      "prikolniy.email@example.com",
	}
	push := &practice.PushNotifier{
		BaseNotifier: practice.BaseNotifier{Channel: "push"},
		DeviceID:     "phone-42",
	}

	err := practice.Dispatch([]practice.Notifier{email, push}, "Заказ готов")
	fmt.Printf("ошибки нет: %t\n", err == nil)
	fmt.Printf("email: %v\n", email.Messages)
	fmt.Printf("push: %v\n", push.Messages)

	fmt.Println("\n=&= Пустой текст =&=")
	beforeEmail := len(email.Messages)
	beforePush := len(push.Messages)
	err = practice.Dispatch([]practice.Notifier{email, push}, "")
	fmt.Printf("errors.Is(ErrEmptyMessage): %t\n", errors.Is(err, practice.ErrEmptyMessage))
	fmt.Printf("истории не изменились: %t\n",
		len(email.Messages) == beforeEmail && len(push.Messages) == beforePush)

	fmt.Println("\n=&= Нет настройки =&=")
	unconfiguredPush := &practice.PushNotifier{
		BaseNotifier: practice.BaseNotifier{Channel: "push"},
	}
	err = practice.Dispatch([]practice.Notifier{unconfiguredPush}, "Заказ готов")
	var sendErr *practice.SendError
	if errors.As(err, &sendErr) {
		fmt.Printf("Channel: %s\n", sendErr.Channel)
		fmt.Printf("Reason: %s\n", sendErr.Reason)
	} else {
		fmt.Printf("*SendError не извлечён: %v\n", err)
	}

	fmt.Println("\n=&= Сломанная реализация =&=")
	recoveredPush := &practice.PushNotifier{
		BaseNotifier: practice.BaseNotifier{Channel: "push"},
		DeviceID:     "phone-2",
	}
	failures := practice.SafeDispatch(
		[]practice.Notifier{&practice.BrokenNotifier{}, recoveredPush},
		"Заказ готов",
	)
	panicFound := false
	for _, failure := range failures {
		if errors.Is(failure, practice.ErrNotifierPanic) {
			panicFound = true
			fmt.Printf("ошибка паники: %v\n", failure)
		}
	}
	fmt.Printf("errors.Is(ErrNotifierPanic): %t\n", panicFound)
	fmt.Printf("push после паники: %v\n", recoveredPush.Messages)
}
