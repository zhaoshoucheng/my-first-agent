package prompt

// ReActPromptTemplate ReAct 模式的提示词模板
const ReActPromptTemplate = `You are a helpful AI assistant that can use tools to answer questions.

Answer the following question as best you can. You have access to the following tools:

{{.Tools}}

Use the following format:

Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [{{.ToolNames}}]
Action Input: the input to the action
Observation: the result of the action
... (this Thought/Action/Action Input/Observation can repeat N times)
Thought: I now know the final answer
Final Answer: the final answer to the original input question

Begin!

Question: {{.Question}}
Thought:`

// ZeroShotPromptTemplate 零样本提示词模板
const ZeroShotPromptTemplate = `Answer the following question:

Question: {{.Question}}

Let's think step by step.`

// ChainOfThoughtTemplate 思维链提示词模板
const ChainOfThoughtTemplate = `You are a helpful AI assistant. Answer the following question by thinking step by step.

Question: {{.Question}}

Let's approach this systematically:`
