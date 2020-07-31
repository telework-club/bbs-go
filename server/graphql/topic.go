package graph

import (
	"bbs-go/config"
	"bbs-go/model"
	"bbs-go/model/constants"
	"bbs-go/services"
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/mlogclub/simple"
)

var (
	TopicType   *graphql.Object
	ForumSchema *graphql.Schema
)

// type TopicViewModel struct {
// 	Id      int
// 	Title   string
// 	Content string
// }

func InitTopicType() {
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
