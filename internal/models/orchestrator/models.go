package models

type CalculateRequest struct {
	Expression string `json:"expression"`
}

type CalculateResponse struct {
	Id int `json:"id"`
}

type ExpressionResponse struct {
	Id     int     `json:"id"`
	Status string  `json:"status"`
	Result float64 `json:"result"`
}

type TaskRequest struct {
}

type TaskResponse struct {
	Login        string  `json:"login"`
	IdExpression int     `json:"id_expression"`
	IdTask       int     `json:"id_task"`
	Arg1         float64 `json:"arg1"`
	Arg2         float64 `json:"arg2"`
	Operation    string  `json:"operation"`
}
