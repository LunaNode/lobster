package i18n

import "github.com/LunaNode/lobster/utils"

import "encoding/json"
import "errors"
import "fmt"
import "io/ioutil"

type Section struct {
	Text map[string]string `json:"T"`
	Errors map[string]string `json:"error"`
	Messages map[string]string `json:"message"`
}

type Language struct {
	Sections map[string]*Section
}

func LoadFile(filename string) (*Language, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	l := new(Language)
	l.Sections = make(map[string]*Section)
	err = json.Unmarshal(contents, &l.Sections)
	if err != nil {
		return nil, err
	} else {
		return l, nil
	}
}

type SectionFunc func(string) *Section

func (this *Language) S(sectionString string) *Section {
	if this.Sections[sectionString] == nil {
		panic("invalid language section " + sectionString)
	} else {
		return this.Sections[sectionString]
	}
}

func (this *Section) T(a ...interface{}) string {
	text := a[0].(string)
	if this.Text[text] != "" {
		text = this.Text[text]
	}

	if len(a) > 1 {
		return fmt.Sprintf(text, a[1:]...)
	} else {
		return text
	}
}

func (this *Section) FormatError(err error) utils.Message {
	return utils.Message{
		Text: fmt.Sprintf(this.Messages["error_format"], err.Error()),
		Type: "danger",
	}
}

func (this *Section) Error(id string) error {
	text, ok := this.Errors[id]
	if !ok {
		return errors.New(id)
	} else {
		return errors.New(text)
	}
}

func (this *Section) Errorf(id string, a ...interface{}) error {
	format, ok := this.Errors[id]
	if !ok {
		return errors.New(id + " " + fmt.Sprint(a...))
	} else {
		return errors.New(fmt.Sprintf(format, a...))
	}
}

func (this *Section) FormattedError(id string) utils.Message {
	return this.FormatError(this.Error(id))
}

func (this *Section) FormattedErrorf(id string, a ...interface{}) utils.Message {
	return this.FormatError(this.Errorf(id, a...))
}

func (this *Section) Message(t string, text string) utils.Message {
	if this.Messages[text] != "" {
		text = this.Messages[text]
	}
	return utils.Message{
		Text: text,
		Type: t,
	}
}

func (this *Section) Messagef(t string, id string, a ...interface{}) utils.Message {
	var text string
	format, ok := this.Messages[id]
	if !ok {
		text = id + " " + fmt.Sprint(a...)
	} else {
		text = fmt.Sprintf(format, a...)
	}
	return utils.Message{
		Text: text,
		Type: t,
	}
}

func (this *Section) Success(text string) utils.Message {
	return this.Message("success", text)
}

func (this *Section) Successf(text string, a ...interface{}) utils.Message {
	return this.Messagef("success", text, a...)
}

func (this *Section) Info(text string) utils.Message {
	return this.Message("info", text)
}

func (this *Section) Infof(text string, a ...interface{}) utils.Message {
	return this.Messagef("info", text, a...)
}
