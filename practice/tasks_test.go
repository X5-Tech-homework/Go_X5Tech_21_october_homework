package practice

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type spy struct {
	name       string
	calls      *[]string
	result     error
	panics     bool
	panicValue any
}

func (s *spy) Name() string {
	return s.name
}

func (s *spy) Send(message string) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, s.name+":"+message)
	}
	if s.panics {
		panic(s.panicValue)
	}
	return s.result
}

type unnamed struct {
	result error
}

func (n unnamed) Send(message string) error {
	return n.result
}

// Помощник не даёт незавершённому SafeSend оборвать весь набор тестов.
func withoutPanic(t *testing.T, action func()) {
	t.Helper()
	defer func() {
		if value := recover(); value != nil {
			t.Fatalf("паника вышла за границу: %v", value)
		}
	}()
	action()
}

func newEmail() *EmailNotifier {
	return &EmailNotifier{
		BaseNotifier: BaseNotifier{Channel: "email"},
		Address:      "student@example.com",
	}
}

func newPush() *PushNotifier {
	return &PushNotifier{
		BaseNotifier: BaseNotifier{Channel: "push"},
		DeviceID:     "phone-1",
	}
}

func TestStep1Sentinel(t *testing.T) {
	if ErrEmptyMessage == nil {
		t.Fatal("создайте ErrEmptyMessage через errors.New")
	}
	if ErrEmptyMessage.Error() != "пустое сообщение" {
		t.Fatalf("неверный текст: %q", ErrEmptyMessage.Error())
	}
}

func TestStep1Channels(t *testing.T) {
	for _, channel := range []string{"email", "push"} {
		t.Run(channel, func(t *testing.T) {
			email, push := newEmail(), newPush()
			var n Notifier = email
			messages := &email.Messages
			if channel == "push" {
				n = push
				messages = &push.Messages
			}
			for _, message := range []string{"Заказ готов", "Ещё заказ", " "} {
				if err := n.Send(message); err != nil {
					t.Fatalf("Send(%q): %v", message, err)
				}
			}
			want := []string{
				"[" + channel + "] Заказ готов",
				"[" + channel + "] Ещё заказ",
				"[" + channel + "]  ",
			}
			if !reflect.DeepEqual(*messages, want) {
				t.Fatalf("история: %v, ожидается %v", *messages, want)
			}
			for i := 0; i < 2; i++ {
				err := n.Send("")
				if err == nil || err != ErrEmptyMessage {
					t.Fatalf("пустой текст: ожидается тот же ErrEmptyMessage, получено %v", err)
				}
			}
			if !reflect.DeepEqual(*messages, want) {
				t.Fatalf("ошибка изменила историю: %v", *messages)
			}
		})
	}
	t.Run("independent_instances", func(t *testing.T) {
		first, second := newEmail(), newEmail()
		if err := first.Send("Первое"); err != nil {
			t.Fatal(err)
		}
		if len(second.Messages) != 0 {
			t.Fatal("истории разных экземпляров смешались")
		}
	})
}

func TestStep2Dispatch(t *testing.T) {
	t.Run("empty_lists", func(t *testing.T) {
		for _, all := range [][]Notifier{nil, {}} {
			if err := Dispatch(all, ""); err != nil {
				t.Fatalf("пустой список: %v", err)
			}
		}
	})
	t.Run("success_and_order", func(t *testing.T) {
		var calls []string
		all := []Notifier{
			&spy{name: "one", calls: &calls},
			&spy{name: "two", calls: &calls},
		}
		if err := Dispatch(all, "hello"); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(calls, []string{"one:hello", "two:hello"}) {
			t.Fatalf("вызовы: %v", calls)
		}
	})
	t.Run("wrap_and_stop", func(t *testing.T) {
		root := errors.New("недоступен канал")
		var calls []string
		all := []Notifier{
			&spy{name: "one", calls: &calls},
			&spy{name: "two", calls: &calls, result: root},
			&spy{name: "three", calls: &calls},
		}
		err := Dispatch(all, "hello")
		if err == nil || err == root || !errors.Is(err, root) {
			t.Fatalf("ожидается обёртка с исходной причиной, получено %v", err)
		}
		if !strings.Contains(err.Error(), "канал two:") {
			t.Fatalf("нет имени канала: %v", err)
		}
		if !reflect.DeepEqual(calls, []string{"one:hello", "two:hello"}) {
			t.Fatalf("нужно остановиться на первой ошибке: %v", calls)
		}
	})
	t.Run("multiple_wraps", func(t *testing.T) {
		root := errors.New("причина")
		inner := fmt.Errorf("внутренний вызов: %w", root)
		err := Dispatch([]Notifier{&spy{name: "email", result: inner}}, "hello")
		if !errors.Is(fmt.Errorf("заказ: %w", err), root) {
			t.Fatal("потеряна причина после нескольких обёрток")
		}
	})
	t.Run("custom_type_preserved", func(t *testing.T) {
		root := &SendError{Channel: "push", Reason: "нет настройки"}
		err := Dispatch([]Notifier{&spy{name: "push", result: root}}, "hello")
		var target *SendError
		if !errors.As(err, &target) || target != root {
			t.Fatalf("потерян исходный *SendError: %v", err)
		}
	})
	t.Run("only_send_method", func(t *testing.T) {
		root := errors.New("причина")
		err := Dispatch([]Notifier{unnamed{result: root}}, "hello")
		if err == nil || !errors.Is(err, root) || !strings.Contains(err.Error(), "practice.unnamed") {
			t.Fatalf("канал без Name тоже должен работать: %v", err)
		}
	})
}

func TestStep3SafeSend(t *testing.T) {
	t.Run("success_once", func(t *testing.T) {
		var calls []string
		n := &spy{name: "email", calls: &calls}
		withoutPanic(t, func() {
			if err := SafeSend(n, "hello"); err != nil {
				t.Fatal(err)
			}
		})
		if !reflect.DeepEqual(calls, []string{"email:hello"}) {
			t.Fatalf("вызовы: %v", calls)
		}
	})
	t.Run("ordinary_error_unchanged", func(t *testing.T) {
		root := errors.New("обычная ошибка")
		if err := SafeSend(&spy{result: root}, "hello"); err != root {
			t.Fatalf("обычная ошибка должна вернуться без замены: %v", err)
		}
	})
	cases := []struct {
		name     string
		value    any
		fragment string
	}{
		{name: "string", value: "поломка", fragment: "поломка"},
		{name: "number", value: 42, fragment: "42"},
		{name: "error", value: errors.New("внутренняя ошибка"), fragment: "внутренняя ошибка"},
		{name: "nil", value: nil},
	}
	for _, tc := range cases {
		t.Run("panic_"+tc.name, func(t *testing.T) {
			withoutPanic(t, func() {
				err := SafeSend(&spy{panics: true, panicValue: tc.value}, "hello")
				if err == nil || !errors.Is(err, ErrNotifierPanic) {
					t.Fatalf("паника не стала известной ошибкой: %v", err)
				}
				if tc.fragment != "" && !strings.Contains(err.Error(), tc.fragment) {
					t.Fatalf("потерян текст паники: %v", err)
				}
			})
		})
	}
	t.Run("broken_notifier", func(t *testing.T) {
		withoutPanic(t, func() {
			err := SafeSend(&BrokenNotifier{}, "hello")
			if !errors.Is(err, ErrNotifierPanic) {
				t.Fatalf("не распознана паника BrokenNotifier: %v", err)
			}
		})
	})
}

func TestStep3SafeDispatch(t *testing.T) {
	t.Run("empty_lists", func(t *testing.T) {
		for _, all := range [][]Notifier{nil, {}} {
			if failures := SafeDispatch(all, ""); len(failures) != 0 {
				t.Fatalf("пустой список: %v", failures)
			}
		}
	})
	t.Run("all_success", func(t *testing.T) {
		var calls []string
		all := []Notifier{&spy{name: "one", calls: &calls}, &spy{name: "two", calls: &calls}}
		if failures := SafeDispatch(all, "hello"); len(failures) != 0 {
			t.Fatalf("лишние ошибки: %v", failures)
		}
		if !reflect.DeepEqual(calls, []string{"one:hello", "two:hello"}) {
			t.Fatalf("вызовы: %v", calls)
		}
	})
	t.Run("continue_after_each_failure", func(t *testing.T) {
		root := errors.New("первая причина")
		custom := &SendError{Channel: "custom", Reason: "нет настройки"}
		var calls []string
		all := []Notifier{
			&spy{name: "ordinary", calls: &calls, result: root},
			&spy{name: "broken", calls: &calls, panics: true, panicValue: "сбой"},
			&spy{name: "custom", calls: &calls, result: custom},
			&spy{name: "good", calls: &calls},
		}
		withoutPanic(t, func() {
			failures := SafeDispatch(all, "hello")
			if len(failures) != 3 {
				t.Fatalf("нужны все три ошибки, получено %v", failures)
			}
			if !errors.Is(failures[0], root) || !errors.Is(failures[1], ErrNotifierPanic) {
				t.Fatalf("причины или порядок потеряны: %v", failures)
			}
			var target *SendError
			if !errors.As(failures[2], &target) || target != custom {
				t.Fatal("потерян *SendError")
			}
			for i, name := range []string{"ordinary", "broken", "custom"} {
				if !strings.Contains(failures[i].Error(), "канал "+name+":") {
					t.Fatalf("ошибка без контекста: %v", failures[i])
				}
			}
		})
		want := []string{"ordinary:hello", "broken:hello", "custom:hello", "good:hello"}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("вызовы: %v, ожидается %v", calls, want)
		}
	})
	t.Run("real_channel_after_panic", func(t *testing.T) {
		push := newPush()
		withoutPanic(t, func() {
			failures := SafeDispatch([]Notifier{&BrokenNotifier{}, push}, "Заказ готов")
			if len(failures) != 1 || !errors.Is(failures[0], ErrNotifierPanic) {
				t.Fatalf("ошибки: %v", failures)
			}
		})
		if !reflect.DeepEqual(push.Messages, []string{"[push] Заказ готов"}) {
			t.Fatalf("следующий канал не получил сообщение: %v", push.Messages)
		}
	})
	t.Run("empty_message_each_channel", func(t *testing.T) {
		email, push := newEmail(), newPush()
		failures := SafeDispatch([]Notifier{email, push}, "")
		if len(failures) != 2 {
			t.Fatalf("каждый канал должен вернуть ошибку: %v", failures)
		}
		for _, err := range failures {
			if err == nil || !errors.Is(err, ErrEmptyMessage) {
				t.Fatalf("причина: %v", err)
			}
		}
		if len(email.Messages) != 0 || len(push.Messages) != 0 {
			t.Fatal("невалидный текст попал в историю")
		}
	})
}

func TestHomeworkSendError(t *testing.T) {
	err := &SendError{Channel: "email", Reason: "не задан адрес"}
	if err.Error() != "канал email: не задан адрес" {
		t.Fatalf("неверное описание: %q", err.Error())
	}
	wrapped := fmt.Errorf("уровень 2: %w", fmt.Errorf("уровень 1: %w", err))
	var target *SendError
	if !errors.As(wrapped, &target) || target != err {
		t.Fatal("не удалось извлечь исходный объект")
	}
}

func TestHomeworkConfiguration(t *testing.T) {
	for _, channel := range []string{"email", "push"} {
		t.Run(channel, func(t *testing.T) {
			email, push := newEmail(), newPush()
			email.Address = ""
			push.DeviceID = ""
			var n Notifier = email
			messages := &email.Messages
			reason := "не задан адрес"
			if channel == "push" {
				n = push
				messages = &push.Messages
				reason = "не задан device id"
			}
			*messages = []string{"старая запись"}
			err := n.Send("hello")
			var target *SendError
			if !errors.As(err, &target) {
				t.Fatalf("нужен *SendError: %v", err)
			}
			if target.Channel != channel || target.Reason != reason {
				t.Fatalf("неверные поля: %+v", target)
			}
			if !reflect.DeepEqual(*messages, []string{"старая запись"}) {
				t.Fatal("ошибка настройки изменила историю")
			}
			err = n.Send("")
			if err == nil || err != ErrEmptyMessage {
				t.Fatalf("пустой текст проверяется раньше настроек: %v", err)
			}
			if !reflect.DeepEqual(*messages, []string{"старая запись"}) {
				t.Fatal("пустой текст изменил историю")
			}
		})
	}
}

func TestRegressionHelpers(t *testing.T) {
	email, push := newEmail(), newPush()
	var nilEmail *EmailNotifier
	var nilPush *PushNotifier
	cases := []struct {
		name        string
		n           Notifier
		address     string
		ok          bool
		description string
	}{
		{"email", email, email.Address, true, "email: " + email.Address},
		{"push", push, "", false, "push: " + push.DeviceID},
		{"nil_interface", nil, "", false, "unknown"},
		{"nil_email", nilEmail, "", false, "unknown"},
		{"nil_push", nilPush, "", false, "unknown"},
		{"other", &spy{}, "", false, "unknown"},
		{"empty_address", &EmailNotifier{}, "", true, "email: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			address, ok := EmailAddress(tc.n)
			if address != tc.address || ok != tc.ok {
				t.Fatalf("EmailAddress = %q, %v", address, ok)
			}
			if got := Describe(tc.n); got != tc.description {
				t.Fatalf("Describe = %q, ожидается %q", got, tc.description)
			}
		})
	}
}
