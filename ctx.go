package main

// ctx.go — всё, что связано с context.Context (пункт 5) и ошибками проверки чисел (пункт 7).

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ctxKey — собственный тип для ключей context.WithValue.
// Если бы ключом была обычная строка "runID", её мог бы случайно
// использовать и другой пакет, и значения бы перепутались.
// Значение ключа типа ctxKey совпадает только с ключом того же типа.
type ctxKey string

const (
	keyRunID    ctxKey = "runID"    // номер запуска поиска (1, 2, 3…)
	keyWorkerID ctxKey = "workerID" // номер воркера (1…NumCPU), 0 — последовательный поиск
)

// ErrBadNumber — ошибка-«метка» для некорректных чисел.
// Её заворачивают через %w, а проверяют через errors.Is(err, ErrBadNumber).
var ErrBadNumber = errors.New("некорректное значение, нужны числа >= 1")

// runIDFrom достаёт номер запуска из контекста.
// ctx.Value возвращает any, поэтому нужно утверждение типа .(int).
// Форма с ok не паникует, если значения нет: тогда вернётся 0.
func runIDFrom(ctx context.Context) int {
	id, ok := ctx.Value(keyRunID).(int)
	if !ok {
		return 0
	}
	return id
}

// workerIDFrom достаёт номер воркера из контекста (0, если его там нет).
func workerIDFrom(ctx context.Context) int {
	id, ok := ctx.Value(keyWorkerID).(int)
	if !ok {
		return 0
	}
	return id
}

// checkNumber проверяет одно число: возвращает «простое или нет» и ошибку.
// Номер запуска и номер воркера она берёт из контекста, а не из параметров:
// это и есть передача данных между функциями через context.
func checkNumber(ctx context.Context, n int) (bool, error) {
	if n < 1 {
		who := "последовательный поиск"
		if w := workerIDFrom(ctx); w > 0 {
			who = fmt.Sprintf("воркер %d", w)
		}
		return false, fmt.Errorf("запуск %d, %s: число %d: %w", runIDFrom(ctx), who, n, ErrBadNumber)
	}
	return isPrimeStupid(n), nil
}

// withTimeout делает дочерний контекст с таймаутом d.
// Если d == 0, таймаута нет, но cancel всё равно возвращается,
// чтобы вызывающий код всегда мог написать defer cancel().
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// reasonText переводит причину отмены контекста в понятное сообщение.
func reasonText(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "прерван пользователем (Ctrl+C)"
	case errors.Is(err, context.DeadlineExceeded):
		return "не уложился в отведённое время"
	default:
		return err.Error()
	}
}
