package main

import (
	"context"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/precheckoutquery"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Kết nối PostgreSQL
func connectDB() (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), "postgres://user:password@localhost:5432/payments_db")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return conn, nil
}

func main() {
	token := ""
	db, err := connectDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(context.Background())

	b, err := gotgbot.NewBot(token, nil)
	if err != nil {
		log.Fatal("failed to create bot:", err)
	}

	dispatcher := ext.NewDispatcher(nil)
	updater := ext.NewUpdater(dispatcher, nil)

	dispatcher.AddHandler(handlers.NewCommand("start", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return start(b, ctx)
	}))
	dispatcher.AddHandler(handlers.NewPreCheckoutQuery(precheckoutquery.All, preCheckout))
	dispatcher.AddHandler(handlers.NewMessage(message.SuccessfulPayment, func(b *gotgbot.Bot, ctx *ext.Context) error {
		return paymentComplete(b, ctx, db)
	}))

	err = updater.StartPolling(b, &ext.PollingOpts{DropPendingUpdates: true})
	if err != nil {
		log.Fatal("failed to start polling:", err)
	}
	log.Printf("%s has been started...\n", b.User.Username)

	updater.Idle()
}

func start(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type != "private" {
		return nil
	}

	_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Hello, I'm @%s. This is a demo payment bot!", b.User.Username), nil)
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}

	transactionID := uuid.NewString()

	_, err = b.SendInvoice(ctx.EffectiveChat.Id, "Product Name", "Detailed product description",
		transactionID, "XTR", []gotgbot.LabeledPrice{{Label: "Product", Amount: 100}},
		&gotgbot.SendInvoiceOpts{ProtectContent: true})

	if err != nil {
		return fmt.Errorf("failed to generate invoice: %w", err)
	}
	return nil
}

func preCheckout(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.PreCheckoutQuery

	// Kiểm tra điều kiện (ví dụ: hết hàng)
	if query.TotalAmount > 1000 {
		_, _ = b.AnswerPreCheckoutQuery(query.Id, false, &gotgbot.AnswerPreCheckoutQueryOpts{
			ErrorMessage: "Giá sản phẩm quá cao, vui lòng chọn sản phẩm khác.",
		})
		return nil
	}

	_, err := b.AnswerPreCheckoutQuery(query.Id, true, nil)
	if err != nil {
		return fmt.Errorf("failed to answer precheckout query: %w", err)
	}
	return nil
}

func paymentComplete(b *gotgbot.Bot, ctx *ext.Context, db *pgx.Conn) error {
	chatID := ctx.EffectiveChat.Id
	username := ctx.EffectiveUser.Username
	amount := ctx.EffectiveMessage.SuccessfulPayment.TotalAmount

	_, err := db.Exec(context.Background(), "INSERT INTO payments (chat_id, username, amount) VALUES ($1, $2, $3)",
		chatID, username, amount)

	if err != nil {
		return fmt.Errorf("failed to insert payment: %w", err)
	}
	_, err = ctx.EffectiveMessage.Reply(b, "✅ Payment completed successfully!", nil)
	return err
}
