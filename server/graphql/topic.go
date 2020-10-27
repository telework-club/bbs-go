package graph

import (
	"bbs-go/config"
	"bbs-go/model"
	"bbs-go/model/constants"
	"bbs-go/services"
	"fmt"

	"github.com/ahmetb/go-linq"
	"github.com/graphql-go/graphql"
	"github.com/mlogclub/simple"
)

var (
	TopicType   *graphql.Object
	ForumSchema *graphql.Schema
)

type ContextKey string

const (
	CtxCommentsType ContextKey = "comments"
	CtxUsersType    ContextKey = "users"
)

type CommentsMap map[int64]model.Comment
type UsersMap map[int64]model.User
type RootType map[string]interface{}

// type TopicViewModel struct {
// 	Id      int
// 	Title   string
// 	Content string
// }

func getSharedData(key ContextKey, id int64, params *graphql.ResolveParams) (dict interface{}, ok bool) {
	dict = nil
	ok = false
	if root, status := params.Info.RootValue.(map[string]interface{}); status {
		key := fmt.Sprintf("%s-%d", key, id)
		dict, ok = root[key]
		return
	}
	return
}

func InitTopicType() {
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "User",
		Description: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.User); ok {
						return model.Id, nil
					}
					return nil, nil
				},
			},
			"nickname": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.User); ok {
						return model.Nickname, nil
					}
					return nil, nil
				},
			},
			"avatar": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.User); ok {
						return model.Avatar, nil
					}
					return nil, nil
				},
			},
		},
	})
	commentType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "Comment",
		Description: "Comment for a topic",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Comment); ok {
						return model.Id, nil
					}
					return nil, nil
				},
			},
			"type": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Comment); ok {
						return model.ContentType, nil
					}
					return nil, nil
				},
			},
			"content": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Comment); ok {
						return model.Content, nil
					}
					return nil, nil
				},
			},
			"user": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(model.Comment); ok {
						if users, ok := getSharedData(CtxUsersType, data.EntityId, &p); ok {
							if uMap, ok := users.(UsersMap); ok {
								if user, ok := uMap[data.UserId]; ok {
									return user, nil
								}
							}
						}
					}
					return nil, nil
				},
			},
		},
	})
	commentType.AddFieldConfig("comments", &graphql.Field{
		Type: commentType,
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			if data, ok := p.Source.(model.Comment); ok {
				if data.QuoteId == 0 {
					return nil, nil
				}
				if comments, ok := getSharedData(CtxCommentsType, data.EntityId, &p); ok {
					if cMap, ok := comments.(CommentsMap); ok {
						if item, ok := cMap[data.QuoteId]; ok {
							return item, nil
						}
					}
				}
			}
			return nil, nil
		},
	})
	TopicType = graphql.NewObject(graphql.ObjectConfig{
		Name:        "Topic",
		Description: "Topic",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Topic); ok {
						return model.Id, nil
					}
					return nil, nil
				},
			},
			"title": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Topic); ok {
						return model.Title, nil
					}
					return nil, nil
				},
			},
			"content": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Topic); ok {
						return model.Content, nil
					}
					return nil, nil
				},
			},
			"link": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Topic); ok {
						return fmt.Sprintf("%s/topic/%d", config.Instance.BaseUrl, model.Id), nil
					}
					return nil, nil
				},
			},
			"like": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if model, ok := p.Source.(model.Topic); ok {
						return model.LikeCount, nil
					}
					return nil, nil
				},
			},
			"comments": &graphql.Field{
				Type: graphql.NewList(commentType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(model.Topic); ok {
						comments := services.CommentService.Find(simple.NewSqlCnd().Eq("entity_id", data.Id))
						commentsMap := make(CommentsMap)
						users := make([]int64, 0)
						linq.From(comments).
							SelectT(func(item model.Comment) linq.KeyValue {
								users = append(users, item.UserId)
								return linq.KeyValue{Key: item.Id, Value: item}
							}).
							ToMap(&commentsMap)
						root := p.Info.RootValue.(map[string]interface{})
						key := fmt.Sprintf("%s-%d", CtxCommentsType, data.Id)
						root[key] = commentsMap
						allUsers := services.UserService.Find(simple.NewSqlCnd().In("id", users))
						usersMap := make(UsersMap)
						linq.From(allUsers).
							SelectT(func(item model.User) linq.KeyValue {
								return linq.KeyValue{Key: item.Id, Value: item}
							}).
							ToMap(&usersMap)
						root[fmt.Sprintf("%s-%d", CtxUsersType, data.Id)] = usersMap
						return comments, nil
					}
					return nil, nil
				},
			},
		},
	})
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"topics": &graphql.Field{
				Type:        graphql.NewList(TopicType),
				Description: "Query topics by node id",
				Args: graphql.FieldConfigArgument{
					"node": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					"onlyLatest": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pageNum := 20
					nodeId := p.Args["node"].(int)
					if l, ok := p.Args["onlyLatest"]; ok && l.(bool) {
						pageNum = 1
					}
					topics, _ := services.TopicService.FindPageByCnd(simple.NewSqlCnd().
						Eq("node_id", nodeId).
						Eq("status", constants.StatusOk).
						Page(1, pageNum).Desc("create_time"))
					// var result []TopicViewModel
					// linq.From(topics).SelectT(func(item model.Topic) TopicViewModel {
					// 	return TopicViewModel{
					// 		Id:      int(item.Id),
					// 		Title:   item.Title,
					// 		Content: item.Content,
					// 	}
					// }).ToSlice(&result)
					return topics, nil
				},
			},
		},
	})
	allSchema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	ForumSchema = &allSchema
}
