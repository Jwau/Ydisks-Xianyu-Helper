package server

// card_image_handlers.go: 图片卡密本地上传图片的保存与预览端点。

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"xianyu-go/internal/logsafe"
)

// cardImageMaxUploadBytes 是本地上传图片允许的最大字节数。
const cardImageMaxUploadBytes int64 = 10 << 20

// cardImageUploadResponse 是上传图片成功后的具名响应。
type cardImageUploadResponse struct {
	// Success 表示上传是否完成。
	Success bool `json:"success"`
	// ImageID 是新图片记录标识，保存图片卡密时通过 image_id 引用。
	ImageID int64 `json:"image_id"`
	// Filename 是检测并清理后的文件名。
	Filename string `json:"filename"`
}

// cardImagesApplication 返回图片卡密上传图片端口。
func (s *Server) cardImagesApplication() CardImagesPort {
	return s.applicationServiceSet().cardImages
}

// uploadCardImage 接收 multipart 图片文件，校验类型与大小后保存为当前用户的图片记录。
func (s *Server) uploadCardImage(w http.ResponseWriter, r *http.Request) {
	// sess 是当前认证会话；上传图片必须归属到具体用户。
	sess := authSess(r)
	if sess == nil {
		writeErr(w, http.StatusUnauthorized, "未授权访问")
		return
	}
	// source、sourceHeader、err 分别是上传文件、multipart 头和读取错误。
	source, sourceHeader, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "缺少图片文件")
		return
	}
	defer source.Close()
	// data、tooLarge、err 是受限读取的图片字节、超限标记和读取错误。
	data, tooLarge, err := readLimitedBytes(source, cardImageMaxUploadBytes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "读取图片失败")
		return
	}
	if tooLarge {
		writeErr(w, http.StatusBadRequest, "图片不能超过 10 MiB")
		return
	}
	// contentType 是按文件内容探测的媒体类型；仅接受 image/*。
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		writeErr(w, http.StatusBadRequest, "文件不是有效图片")
		return
	}
	// filename 是清理后的展示文件名；空文件名回退为固定名称。
	filename := safeBaseName(sourceHeader.Filename)
	if filename == "" {
		filename = "upload.png"
	}
	// imageID、err 是新图片记录标识和保存错误。
	imageID, err := s.cardImagesApplication().Create(r.Context(), sess.UserID, filename, contentType, data)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("保存上传图片失败", "user_id", sess.UserID, "err", logsafe.Error(err))
		}
		writeErr(w, http.StatusInternalServerError, "保存图片失败")
		return
	}
	writeJSON(w, http.StatusOK, cardImageUploadResponse{Success: true, ImageID: imageID, Filename: filename})
}

// getCardImage 在归属校验后输出上传图片的字节，供前端编辑弹窗预览。
func (s *Server) getCardImage(w http.ResponseWriter, r *http.Request) {
	// sess 是当前认证会话；图片只能被上传者本人读取。
	sess := authSess(r)
	if sess == nil {
		writeErr(w, http.StatusUnauthorized, "未授权访问")
		return
	}
	// imageID、parseErr 是路径中的图片标识及解析错误。
	imageID, parseErr := strconv.ParseInt(chi.URLParam(r, "image_id"), 10, 64)
	if parseErr != nil {
		writeErr(w, http.StatusBadRequest, "无效图片ID")
		return
	}
	// found 表示图片是否存在且属于当前用户。
	found, filename, contentType, data, err := s.cardImagesApplication().GetForUser(r.Context(), sess.UserID, imageID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取图片失败")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "图片不存在")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
