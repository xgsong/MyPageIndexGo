package prompts

import "fmt"

const TOCDetectorPromptTmpl = `Your job is to detect if there is a table of content provided in the given text.

Given text: %s

return the following JSON format:
{
    "toc_detected": "yes or no"
}

Directly return the final JSON structure. Do not output anything else.
Please note: abstract, summary, notation list, figure list, table list, etc. are not table of contents.`

func TOCDetectorPrompt(content string) string {
	return fmt.Sprintf(TOCDetectorPromptTmpl, content)
}

const TOCTransformerPromptTmpl = `你现在需要将给定的目录内容转换为标准的JSON格式。
请严格按照以下要求返回结果：
1. 只返回JSON格式内容，不要任何其他解释、说明、或者额外文本
2. JSON结构必须严格符合下面的格式要求：
{
    "table_of_contents": [
        {
            "structure": "目录层级编号，字符串类型，比如"1", "1.1", "2.3.1"，没有则填"None"",
            "title": "章节标题，字符串类型",
            "page": 页码，数字类型，没有则填null
        }
    ]
}
3. 确保JSON格式正确，没有语法错误

现在要转换的目录内容是：
%s`

func TOCTransformerPrompt(tocContent string) string {
	return fmt.Sprintf(TOCTransformerPromptTmpl, tocContent)
}

const TOCIndexExtractorPromptTmpl = `You are given a table of contents in a json format and several pages of a document, your job is to add the physical_index to the table of contents in the json format.

The provided pages contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

The structure variable is the numeric system which represents the index of the hierarchy section in the table of contents. For example, the first section has structure index 1, the first subsection has structure index 1.1, the second subsection has structure index 1.2, etc.

The response should be in the following JSON format:
[
    {
        "structure": "structure index, x.x.x or None (string)",
        "title": "title of the section",
        "physical_index": "<physical_index_X>" (keep the format)
    }
]

Only add the physical_index to the sections that are in the provided pages.
If the section is not in the provided pages, do not add the physical_index to it.
Directly return the final JSON structure. Do not output anything else.

Table of contents:
%s

Document pages:
%s`

func TOCIndexExtractorPrompt(tocJSON, content string) string {
	return fmt.Sprintf(TOCIndexExtractorPromptTmpl, tocJSON, content)
}

const TOCCompletenessCheckPromptTmpl = `请检查整理后的目录是否完整，包含了原始目录的所有内容。
请严格按照JSON格式返回结果，不要任何其他内容：
{
    "completed": "yes或者no"
}

原始目录内容：
%s

整理后的目录内容：
%s`

func TOCCompletenessCheckPrompt(rawContent, transformedContent string) string {
	return fmt.Sprintf(TOCCompletenessCheckPromptTmpl, rawContent, transformedContent)
}

const TOCContinuePromptTmpl = `Your task is to continue the table of contents json structure, directly output the remaining part of the json structure.

The raw table of contents json structure is:
%s

The incomplete transformed table of contents json structure is:
%s

Please continue the json structure, directly output the remaining part of the json structure.`

func TOCContinuePrompt(rawContent, incompleteContent string) string {
	return fmt.Sprintf(TOCContinuePromptTmpl, rawContent, incompleteContent)
}
