package main

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/remoteconfig"
	"google.golang.org/api/option"
)

type abc struct {
	Name string
	Age  int
}

func main() {
	ctx := context.Background()

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile("key.json"))
	if err != nil {
		log.Fatalf("[Main] Error in creating app : %v ", err)
	}

	rc, rcErr := app.RemoteConfig(ctx)
	if rcErr != nil {
		log.Fatalf("[Main] Error in creating RC client : %v ", rcErr)
	}

	fmt.Println("rc client ", rc)

	// ************* ONE ****************
	// Operations on an empty template, using GetServerTemplate
	// emptyTemplateOperations(ctx, rc)

	// ************* TWO ****************
	// Template evaluation with different conditions
	// templateEvaluation(ctx, rc)

	// ************* THREE ****************
	// Get and InitServerTemplate on a non-empty template
	// fetchRemoteAndLocalTemplate(ctx, rc)

}

func emptyTemplateOperations(ctx context.Context, rc *remoteconfig.Client) {
	// empty default config
	defaultConfig := make(map[string]any)
	template, tErr := rc.GetServerTemplate(ctx, defaultConfig)
	if tErr != nil {
		log.Fatalf("[emptyTemplateOperations] Error in fetching template : %v ", tErr)
	}

	// empty context
	context := make(map[string]any)
	config, cErr := template.Evaluate(context)

	jsonifiedTemplate, _ := template.ToJSON()
	fmt.Println("[emptyTemplateOperations] jsonified template : ", jsonifiedTemplate)

	if cErr != nil {
		log.Fatalf("[emptyTemplateOperations] Error in evaluating template : %v ", cErr)
	}
	// config should be empty
	fmt.Println("[emptyTemplateOperations] config is ", config)
}

func templateEvaluation(ctx context.Context, rc *remoteconfig.Client) {
	// this is how you create an object
	// abcObj := abc{Name : "jane", Age: 20}

	// pass values based on your template
	defaultConfig := make(map[string]any)

	// to pass default parameter values
	// defaultConfig["paramName"] = "paramValue"

	template, tErr := rc.GetServerTemplate(ctx, defaultConfig)
	if tErr != nil {
		log.Fatalf("[templateEvaluation] Error in fetching template : %v ", tErr)
	}

	// pass values based on your template
	context := make(map[string]any)

	// to pass a randomizationID
	// context["randomizationID"] = "12345"

	config, cErr := template.Evaluate(context)
	if cErr != nil {
		log.Fatalf("[templateEvaluation] Error in evaluating template : %v ", cErr)
	}

	// to check for Static  / Default value sources
	// fmt.Println("Value source is Static : ", config.GetValueSource(paramOne) == remoteconfig.Static)
	// fmt.Println("Value source is Default : ", config.GetValueSource(paramOne) == remoteconfig.Default)

	// Repeat this block for all your parameters, (use := to create new variables)
	paramOne := "param_one"
	fmt.Println("Value source is Remote : ", config.GetValueSource(paramOne) == remoteconfig.Remote)
	fmt.Println("as Boolean ", config.GetBoolean(paramOne))
	fmt.Println("as Float ", config.GetFloat(paramOne))
	fmt.Println("as Int ", config.GetInt(paramOne))
	fmt.Println("as String ", config.GetString(paramOne))
}

func fetchRemoteAndLocalTemplate(ctx context.Context, rc *remoteconfig.Client) {
	// empty default config
	defaultConfig := make(map[string]any)

	template, tErr := rc.GetServerTemplate(ctx, defaultConfig)
	if tErr != nil {
		log.Fatalf("[fetchRemoteAndLocalTemplate] Error in fetching template : %v ", tErr)
	}

	// validate the JSON received
	jsonifiedTemplate, _ := template.ToJSON()
	fmt.Println("[fetchRemoteAndLocalTemplate] jsonified template : ", jsonifiedTemplate)

	// modify the value of the jsonified template, try a valid and invalid case
	templateToInit := jsonifiedTemplate
	templateTwo, tTwoErr := rc.InitServerTemplate(defaultConfig, templateToInit)
	if tTwoErr != nil {
		log.Fatalf("[fetchRemoteAndLocalTemplate] Error in fetching template two : %v ", tTwoErr)
	}

	// validate that you receive valid JSON
	jsonifiedTemplateTwo, _ := templateTwo.ToJSON()
	fmt.Println("[fetchRemoteAndLocalTemplate] jsonified template two : ", jsonifiedTemplateTwo)
}
