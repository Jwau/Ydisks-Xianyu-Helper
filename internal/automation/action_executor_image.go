// action_executor_image.go: 图片卡密本地上传图片的发送职责，与 URL 模式共享发送器边界。
package automation

import (
	"context"
	"fmt"

	"xianyu-go/internal/db"
)

// imageByteSender 由支持发送本地上传图片字节的发送器选择性实现。
type imageByteSender interface {
	// SendImageData 上传图片字节到闲鱼图片服务并发送给指定买家。
	SendImageData(ctx context.Context, chatID, toUserID, filename, contentType string, data []byte, cardID int64) error
}

// sendUploadedImageCard 发送绑定本地上传图片的图片卡密；按购买数量逐单位上传并发送。
func (e *automationActionExecutor) sendUploadedImageCard(ctx context.Context, task Task, card *db.CardFull, count int) (int, error) {
	// image、imgErr 是本地上传图片的字节和读取错误；读取在动作上下文内一次性完成。
	image, imgErr := e.store.Cards.GetImageForUser(ctx, card.UserID, card.ImageID)
	if imgErr != nil {
		return 0, fmt.Errorf("读取上传图片: %w", imgErr)
	}
	// sentUnits 是本地上传图片模式下已成功发送的数量。
	sentUnits := 0
	// i 表示当前卡密发送序号。
	for i := 0; i < count; i++ {
		// sendErr 保存上传并发送图片的错误；上传发生在 WebSocket 写入之前。
		if sendErr := e.sendImageData(ctx, task, image.Filename, image.ContentType, image.Data, card.ID); sendErr != nil {
			return sentUnits, classifyMessageSendError(sendErr)
		}
		sentUnits++
	}
	return sentUnits, nil
}

// sendImageData 上传本地上传图片的字节并按卡密发送；上传失败发生在 WebSocket 写入之前，标记确定未发送。
func (e *automationActionExecutor) sendImageData(ctx context.Context, task Task, filename, contentType string, data []byte, cardID int64) error {
	if task.ChatID == "" || task.BuyerID == "" {
		return fmt.Errorf("%w: 发送图片缺少 chat_id 或 buyer_id", ErrMessageNotSent)
	}
	if e.senders == nil {
		return fmt.Errorf("%w: 账号发送器未初始化", ErrMessageNotSent)
	}
	// sender、senderOK 分别是当前账号的在线图片发送器及其可用标记，账号离线时必须返回确定未发送错误。
	sender, senderOK := e.senders.Sender(task.AccountID)
	if !senderOK {
		return fmt.Errorf("%w: 账号未在线，无法发送自动化图片", ErrMessageNotSent)
	}
	// byteSender 是支持字节上传的选择性接口；未实现时明确报错而不是静默降级。
	byteSender, ok := sender.(imageByteSender)
	if !ok {
		return fmt.Errorf("%w: 图片发送器不支持本地上传图片", ErrMessageNotSent)
	}
	return byteSender.SendImageData(ctx, task.ChatID, task.BuyerID, filename, contentType, data, cardID)
}
