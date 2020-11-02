package graph

import (
	"bbs-go/config"
	"bbs-go/model"
	"bbs-go/model/constants"
	"bbs-go/services"
	"fmt"

	"github.com/88250/lute"
	"github.com/ahmetb/go-linq"
	"github.com/graphql-go/graphql"
	"github.com/mlogclub/simple"
	"github.com/spf13/cast"
)

var (
	TopicType   *graphql.Object
	ForumSchema *graphql.Schema
)

type ContextKey string

const (
	CtxCommentsType ContextKey = "comments"
	CtxUsersType    ContextKey = "users"
	CtxTopicType    string     = "topic-content-type"
	UserCache       string     = "user-cache"
)

type CommentsMap map[int64]model.Comment
type UsersMap map[int64]model.User
type RootType map[string]interface{}

// type TopicViewModel struct {
// 	Id      int
// 	Title   string
// 	Content string
// }

func chooseContentType(p *graphql.ResolveParams, content string) string {
	root := p.Info.RootValue.(map[string]interface{})
	contentType := cast.ToInt(root[CtxTopicType])
	switch contentType {
	case 1:
		luteEngine := lute.New()
		return luteEngine.MarkdownStr("topic", content)
	default:
		return content
	}
}

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

func queryDataFromCache(params *graphql.ResolveParams, key string, id int64) (data interface{}, ok bool) {
	if root, status := params.Info.RootValue.(map[string]interface{}); status {
		if result, in := root[key]; in {
			cache := result.(*LazyQuery)
			dataInCache := cache.Get(id)
			if len(dataInCache) > 0 {
				ok = true
				data = dataInCache[0]
				return
			}
		}
	}
	return
}

func setKeysToCache(params *graphql.ResolveParams, key string, ids ...int64) {
	if root, status := params.Info.RootValue.(map[string]interface{}); status {
		if result, in := root[key]; in {
			cache := result.(*LazyQuery)
			cache.Set(ids...)
		}
	}
}

type LazyQueryFn func(ids ...int64) map[int64]interface{}

type LazyQuery struct {
	ids          []int64
	noQueriedIds []int64
	cache        map[int64]interface{}
	Query        LazyQueryFn
}

func (query *LazyQuery) Set(ids ...int64) {
	newQuery := linq.From(ids)
	current := linq.From(query.ids)
	notIn := newQuery.Except(current)
	all := current.Union(notIn)
	currentWaiting := linq.From(query.noQueriedIds)
	noQuery := currentWaiting.Union(notIn)
	all.Distinct().ToSlice(&query.ids)
	noQuery.Distinct().ToSlice(&query.noQueriedIds)
}

func (query *LazyQuery) Get(ids ...int64) []interface{} {
	result := make([]interface{}, 0)
	idsQuery := linq.From(ids)
	currentWaiting := linq.From(query.noQueriedIds)
	needQuery := idsQuery.Intersect(currentWaiting)
	if needQuery.Count() != 0 {
		var need []int64
		needQuery.ToSlice(&need)
		queryResult := query.Query(query.noQueriedIds...)
		query.noQueriedIds = make([]int64, 0)
		for key, value := range queryResult {
			query.cache[key] = value
		}

	}
	for _, id := range ids {
		if item, ok := query.cache[id]; ok {
			result = append(result, item)
		}
	}
	return result
}

type QueryCache interface {
	Get(ids ...int64) []interface{}
	Set(ids ...int64)
}

func initLazyQuery(query LazyQueryFn) *LazyQuery {
	lq := LazyQuery{
		cache:        make(map[int64]interface{}),
		Query:        query,
		ids:          make([]int64, 0),
		noQueriedIds: make([]int64, 0),
	}
	return &lq
}

func InitTopicType() {
	topicContentTypeEnum := graphql.NewEnum(graphql.EnumConfig{
		Name: "TopicContentType",
		Values: graphql.EnumValueConfigMap{
			"MD": &graphql.EnumValueConfig{
				Value: 0,
			},
			"HTML": &graphql.EnumValueConfig{
				Value: 1,
			},
		},
	})
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "User",
		Description: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:    graphql.Int,
				Resolve: modelFieldResolver("Id"),
			},
			"nickname": &graphql.Field{
				Type:    graphql.String,
				Resolve: modelFieldResolver("Nickname"),
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
	topicNodeType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "TopicNode",
		Description: "Topic node",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:    graphql.Int,
				Resolve: modelFieldResolver("id"),
			},
			"name": &graphql.Field{
				Type:    graphql.String,
				Resolve: modelFieldResolver("Name"),
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
						return chooseContentType(&p, model.Content), nil
					}
					return nil, nil
				},
			},
			"user": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(model.Comment); ok {
						if user, ok := queryDataFromCache(&p, UserCache, data.UserId); ok {
							return user, nil
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
						return chooseContentType(&p, model.Content), nil
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
			"user": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(model.Topic); ok {
						if user, ok := queryDataFromCache(&p, UserCache, data.UserId); ok {
							return user, nil
						}
					}
					return nil, nil
				},
			},
			"comments": &graphql.Field{
				Type: graphql.NewList(commentType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(model.Topic); ok {
						comments := services.CommentService.Find(simple.NewSqlCnd().Eq("entity_id", data.Id))
						if len(comments) == 0 {
							return nil, nil
						}
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
						setKeysToCache(&p, UserCache, users...)
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
					return topics, nil
				},
			},
			"recentlyTopics": &graphql.Field{
				Type:        graphql.NewList(TopicType),
				Description: "Query recent 10 topics ",
				Args: graphql.FieldConfigArgument{
					"page": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					"type": &graphql.ArgumentConfig{
						Type:         topicContentTypeEnum,
						DefaultValue: "MD",
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pageNum := 10
					page := p.Args["page"].(int)
					if page <= 0 {
						page = 1
					}
					topics := services.TopicService.Find(simple.NewSqlCnd().Where("status = ?", constants.StatusOk).Desc("id").Page(page, pageNum))
					root := p.Info.RootValue.(map[string]interface{})
					userCache := initLazyQuery(func(ids ...int64) map[int64]interface{} {
						users := make(map[int64]interface{})
						allUsers := services.UserService.Find(simple.NewSqlCnd().In("id", ids))
						linq.From(allUsers).SelectT(func(user model.User) linq.KeyValue { return linq.KeyValue{Key: user.Id, Value: user} }).ToMap(&users)
						return users
					})
					var userIds []int64
					linq.From(topics).SelectT(func(topic model.Topic) int64 { return topic.UserId }).ToSlice(&userIds)
					userCache.Set(userIds...)
					root[UserCache] = userCache
					root[CtxTopicType] = p.Args["type"]
					return topics, nil
				},
			},
			"topicNodes": &graphql.Field{
				Type:        graphql.NewList(topicNodeType),
				Description: "query topic node",
				Args: graphql.FieldConfigArgument{
					"editable": &graphql.ArgumentConfig{
						Type:         graphql.Boolean,
						DefaultValue: false,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					isOnlyEditable := p.Args["editable"].(bool)
					if isOnlyEditable {

					}
					return services.TopicNodeService.GetNodes(), nil
				},
			},
		},
	})
	allSchema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
		Types: []graphql.Type{
			topicContentTypeEnum,
		},
	})
	ForumSchema = &allSchema
}
