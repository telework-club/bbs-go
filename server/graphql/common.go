package graph

import (
	"reflect"

	"github.com/graphql-go/graphql"
)

type ContextKey string

const (
	CtxCommentsType ContextKey = "comments"
	CtxUsersType    ContextKey = "users"
	CtxCurrentUser  ContextKey = "current-user"
	CtxTopicType    string     = "topic-content-type"
	UserCache       string     = "user-cache"
)

type RequestOptions struct {
	Query         string                 `json:"query" url:"query" schema:"query"`
	Variables     map[string]interface{} `json:"variables" url:"variables" schema:"variables"`
	OperationName string                 `json:"operationName" url:"operationName" schema:"operationName"`
}

func modelFieldResolver(fieldName string) graphql.FieldResolveFn {
	field := fieldName
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := reflect.ValueOf(p.Source)
		value := data.FieldByName(field)
		if value.IsValid() {
			return value.Interface(), nil
		}
		return nil, nil
	}
}

type ModelResolvers map[string]graphql.FieldResolveFn

type GraphqlModel interface {
	GetModelName() string
	GetModelDescription() string
	GetResolvers() ModelResolvers
}

// const gqTagname = "gq"

// func convertName(name string) string {
// 	if len(name) < 1 {
// 		return name
// 	}
// 	return strings.ToLower(name[0:1]) + name[1:]
// }

// func GenerateModelSchema(model GraphqlModel) *graphql.Object {
// 	fields := make(graphql.Fields)
// 	modelType := reflect.TypeOf(model)
// 	length := modelType.NumField()
// 	resolvers := model.GetResolvers()
// 	for i := 0; i < length; i++ {
// 		isArray := false
// 		field := modelType.Field(i)
// 		fieldType := graphql.String
// 		switch field.Type.Kind() {
// 		case reflect.Bool:
// 			fieldType = graphql.Boolean
// 		case reflect.Uint, reflect.Uint16, reflect.Uint8, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
// 			fieldType = graphql.Int
// 		case reflect.Float32, reflect.Float64:
// 			fieldType = graphql.Float

// 		case reflect.Slice:
// 			isArray = true
// 		}

// 		fieldName := convertName(field.Name)
// 		gqField := graphql.Field{
// 			Name:        fieldName,
// 			Description: fieldName,
// 			Resolve:     modelFieldResolver(field.Name),
// 			Type:        fieldType,
// 		}

// 		attachedName := fieldName
// 		if tags, ok := field.Tag.Lookup(gqTagname); ok {
// 			if tags == "-" {
// 				continue
// 			}
// 			items := strings.Split(tags, ";")
// 			for _, item := range items {
// 				kv := strings.Split(item, ":")
// 				kvLen := len(kv)
// 				if kvLen == 2 {
// 					switch kv[0] {
// 					case "name":
// 						gqField.Name = kv[1]
// 					case "desc":
// 						gqField.Description = kv[1]
// 					case "resolver":
// 						if resolver, ok := resolvers[kv[1]]; ok {
// 							gqField.Resolve = resolver
// 						}
// 					case "field":
// 						attachedName = kv[1]
// 					case "type":
// 					}
// 				}
// 			}
// 			fields[attachedName] = &gqField
// 		} else {

// 		}
// 	}
// }
