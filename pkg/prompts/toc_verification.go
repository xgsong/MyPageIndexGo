package prompts

import "fmt"

const TitleAppearancePromptTmpl = `Your job is to check if the given section appears or starts in the given page_text.

Note: do fuzzy matching, ignore any space inconsistency in the page_text.

The given section title is %s.
The given page_text is %s.

Reply format:
{
    "answer": "yes or no"
}
Directly return the final JSON structure. Do not output anything else.`

func TitleAppearancePrompt(title, pageText string) string {
	return fmt.Sprintf(TitleAppearancePromptTmpl, title, pageText)
}

const TitleAppearanceInStartPromptTmpl = `You will be given the current section title and the current page_text.
Your job is to check if the current section starts in the beginning of the given page_text.
If there are other contents before the current section title, then the current section does not start in the beginning of the given page_text.
If the current section title is the first content in the given page_text, then the current section starts in the beginning of the given page_text.

Note: do fuzzy matching, ignore any space inconsistency in the page_text.

The given section title is %s.
The given page_text is %s.

reply format:
{
    "start_begin": "yes or no"
}
Directly return the final JSON structure. Do not output anything else.`

func TitleAppearanceInStartPrompt(title, pageText string) string {
	return fmt.Sprintf(TitleAppearanceInStartPromptTmpl, title, pageText)
}

const AddPageNumberToTOCPromptTmpl = `You are given an JSON structure of a document and a partial part of the document. Your task is to check if the title that is described in the structure is started in the partial given document.

The provided text contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

If the full target section starts in the partial given document, insert the given JSON structure with the "start": "yes", and "start_index": "<physical_index_X>".

If the full target section does not start in the partial given document, insert "start": "no",  "start_index": None.

The response should be in the following format.
    [
        {
            "structure": "structure index, x.x.x or None (string)",
            "title": "title of the section",
            "start": "yes or no",
            "physical_index": "<physical_index_X> (keep the format)" or None
        },
        ...
    ]
The given structure contains the result of the previous part, you need to fill the result of the current part, do not change the previous result.
Directly return the final JSON structure. Do not output anything else.

Current Partial Document:
%s

Given Structure
%s`

func AddPageNumberToTOCPrompt(content, structureJSON string) string {
	return fmt.Sprintf(AddPageNumberToTOCPromptTmpl, content, structureJSON)
}

const FindSectionLocationPromptTmpl = `You are given a section title and several pages of a document, your job is to find the physical index of the start page of the section in the partial document.

The provided pages contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

Reply in a JSON format:
{
    "physical_index": "<physical_index_X> (keep the format)"
}
Directly return the final JSON structure. Do not output anything else.

Section Title: %s
Document pages: %s`

func FindSectionLocationPrompt(title, content string) string {
	return fmt.Sprintf(FindSectionLocationPromptTmpl, title, content)
}

const SingleTOCItemIndexFixerPromptTmpl = `You are given an JSON structure of a document and a partial part of the document. Your task is to check if the title that is described in the structure is started in the partial given document.

The provided text contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

If the full target section starts in the partial given document, insert the given JSON structure with the "start": "yes", and "start_index": "<physical_index_X>".

If the full target section does not start in the partial given document, insert "start": "no", "start_index": None.

The response should be in the following format.
    [
        {
            "structure": "structure index",
            "title": "title of the section",
            "start": "yes or no",
            "physical_index": "<physical_index_X> (keep the format)" or None
        }
    ]
Directly return the final JSON structure. Do not output anything else.

Current Partial Document:
%s

Given Structure
%s`

func SingleTOCItemIndexFixerPrompt(content, itemJSON string) string {
	return fmt.Sprintf(SingleTOCItemIndexFixerPromptTmpl, content, itemJSON)
}
