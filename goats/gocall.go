package goats

import (
	"code.google.com/p/go.net/html"
	"fmt"
	"io"
	"strings"
)

type Replacement struct {
	Name string
	Head Processor
	Args []*Argument
}

type GoCallProcessor struct {
	BaseProcessor
	pkgAlias     string
	templateName string
	args         []*Argument
	replacements []*Replacement
	callerAttrs  []html.Attribute
}

func (c *GoCallProcessor) Process(writer io.Writer, context *TagContext) {
	var argType string
	var newTemplateName string
	if c.pkgAlias == "" {
		// In-package template call.
		argType = fmt.Sprintf("%sTemplateArgs", c.templateName)
		newTemplateName = fmt.Sprintf("New%sTemplate", c.templateName)
	} else {
		argType = fmt.Sprintf("%s.%sTemplateArgs", c.pkgAlias, c.templateName)
		newTemplateName = fmt.Sprintf("%s.New%sTemplate", c.pkgAlias, c.templateName)
	}

	// Start of local scope
	io.WriteString(writer, "{\n")

	io.WriteString(writer, fmt.Sprintf("__args := &%s {\n", argType))
	for _, argDef := range c.args {
		expr := context.RewriteExpression(argDef.Val)
		io.WriteString(writer, ToPublicName(argDef.Name)+": "+expr+",\n")
	}
	io.WriteString(writer, "}\n")

	// Call template.
	io.WriteString(writer,
		fmt.Sprintf("__tplt := %s(__impl.GetWriter(), __impl.GetSettings())\n", newTemplateName))
	// Caller Attributes.
	if c.callerAttrs != nil {
		io.WriteString(writer, "__tplt.SetCallerAttrsFunc(func() (runtime.TagAttrs, bool, bool) {\n")
		io.WriteString(writer, "__callerAttrs := runtime.TagAttrs{}\n")
		io.WriteString(writer, "var __hasOmitTag bool\n")
		io.WriteString(writer, "var __omitTag bool\n")
		for _, attr := range c.callerAttrs {
			if attr.Key == "go:omit-tag" {
				io.WriteString(writer, "__hasOmitTag = true\n")
				io.WriteString(writer, fmt.Sprintf("__omitTag = %s\n", context.RewriteExpression(attr.Val)))
			} else if attr.Key == "go:attr" {
				varName, varVal := SplitVarDef(attr.Val)
				expr := context.RewriteExpression(varVal)
				io.WriteString(writer, fmt.Sprintf("__callerAttrs.AddAttr(\"%s\", %s)\n", varName, expr))
			} else if !strings.HasPrefix(attr.Key, "go:") {
				// Static attributes
				io.WriteString(writer,
					fmt.Sprintf("__callerAttrs.AddAttr(\"%s\", \"%s\")\n", attr.Key, attr.Val))
			}
		}
		io.WriteString(writer, "return __callerAttrs, __hasOmitTag, __omitTag\n")
		io.WriteString(writer, "})\n")
	}
	// Replacements.
	for _, replacement := range c.replacements {
		argType := fmt.Sprintf("%s%sReplArgs", c.templateName, replacement.Name)
		if c.pkgAlias == "" {
			io.WriteString(writer,
				fmt.Sprintf("  __tplt.Replace%s(func(__args *%s) {\n", replacement.Name, argType))
		} else {
			io.WriteString(writer,
				fmt.Sprintf("  __tplt.Replace%s(func(__args *%s.%s) {\n",
					replacement.Name, c.pkgAlias, argType))
		}

		for _, arg := range replacement.Args {
			io.WriteString(writer, fmt.Sprintf("  %s := __args.%s\n", arg.Name, ToPublicName(arg.Name)))
		}
		replacement.Head.Process(writer, context)

		io.WriteString(writer, "})\n")
	}
	io.WriteString(writer, "__tplt.Render(__args)\n")

	// Start of local scope.
	io.WriteString(writer, "}\n")

	// go:call is a terminal processor.
}

func NewCallProcessor(pkgAlias string, templateName string, args []*Argument,
	replacements []*Replacement, callerAttrs []html.Attribute) *GoCallProcessor {
	processor := &GoCallProcessor{
		pkgAlias:     strings.Replace(pkgAlias, ".", "_", -1),
		templateName: templateName,
		args:         args,
		replacements: replacements,
		callerAttrs:  callerAttrs,
	}
	return processor
}