package conversation

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/conversations"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"go.uber.org/zap"
)

type Adapter struct {
	client *openai.Client
	logger *zap.SugaredLogger
}

// New создаёт адаптер диалогов.
func New(client *openai.Client, logger *zap.SugaredLogger) *Adapter {
	return &Adapter{client: client, logger: logger}
}

// NewConversation создаёт диалог и возвращает его ID.
// На вход принимает systemText (role=system) и contextText (role=user)
func (a *Adapter) NewConversation(ctx context.Context, systemText string, contextText string, metadata map[string]string) (string, error) {
	params := conversations.ConversationNewParams{}
	items := make([]responses.ResponseInputItemUnionParam, 0, 2)
	if systemText != "" {
		items = append(items,
			responses.ResponseInputItemParamOfMessage(
				responses.ResponseInputMessageContentListParam{
					{OfInputText: &responses.ResponseInputTextParam{Text: systemText}},
				},
				responses.EasyInputMessageRoleSystem,
			),
		)
	}
	items = append(items,
		responses.ResponseInputItemParamOfMessage(
			responses.ResponseInputMessageContentListParam{
				{OfInputText: &responses.ResponseInputTextParam{Text: contextText}},
			},
			responses.EasyInputMessageRoleUser,
		),
	)
	if len(items) > 0 {
		params.Items = items
	}
	if len(metadata) > 0 {
		params.Metadata = shared.Metadata(metadata)
	}

	//start := time.Now()
	//a.logger.Infow("Создание OpenAI диалога...")
	conv, err := a.client.Conversations.New(ctx, params)
	/*dur := time.Since(start)
	if err != nil {
		a.logger.Errorw("Ошибка создания диалога OpenAI", "duration", dur.String(), "error", err)
	} else {
		a.logger.Infow("Диалог OpenAI создан", "duration", dur.String())
	}*/
	if err != nil {
		return "", err
	}

	return conv.ID, nil
}
