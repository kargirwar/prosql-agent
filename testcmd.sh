#DSN=server:dev-server@tcp(127.0.0.1:3306)/test-generico go test -v -race session_test.go session.go sessionhandler.go cursor.go constants.go
#DSN=server:dev-server@tcp(127.0.0.1:3306)/test-generico go test -v -run Execute session_test.go utils.go session.go sessionhandler.go cursor.go constants.go
find . -name "*.go" | entr -r -s 'go run main.go routes.go cursor.go session.go sessionhandler.go'
