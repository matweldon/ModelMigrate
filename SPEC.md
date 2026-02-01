# ModelMigrate

This project builds an agentic AI microservice to manage (audit, document and convert) economic, policy, or accounting models contained in bloated, complex Excel workbooks.

The product will enable civil servants, consultants and other professionals who deal with complex Excel models to:

* Generate documentation for the workbook
* Diagnose and fix errors in the spreadsheet representation of the model
* Convert the model into a reproducible Python representation, augmented by an interactive interface

The big idea here is that there are still too many decision-making, scenario, and policy models implemented in Excel. Many of them are legacy and are too difficult for their current owners to understand and update/convert. AI agents provide an opportunity to do something about this technical debt. However, AI works best when it is constrained by a strong structured framework to verify and focus its reasoning. For that reason, this project will develop an intermediate data structure that the AI can manipulate and reason about.

The product will eventually be an architecture of agentic AI microservice(s) running on a robust, asyncronous cloud architecture to autonomously process Excel models.
At the core of the product will be a suite of tools that parse Excel workbooks to an intermediate data structure (according to a deterministic algorithm), and an agentic harness that gives an AI agent the capabilities to manipulate the intermediate data, to interpret, diagnose and convert to other formats. The agentic harness will also run checks to verify that the outputs maintain fidelity to the inputs.

This repo will be a monorepo that may eventually contain more than one independent package or microservice, including:

* Excel workbook parsing algorithm and microservice: The workbook is parsed to an intermediate representation deterministically to avoid hallucinations.
* AI agent to process the intermediate representation, performing operations such as merging and splitting arrays that preserve the structure's isomorphism while improving its interpretability. The AI agent can then perform a variety of tasks such as auditing, updating data, diagnosing errors, and converting to Python models. The AI agent has tools such as workbook reader and screenshot, that enable it to interpret workbooks
* Verifiable fidelity: the AI agent is constrained by the intermediate data structure
* Interactive notebook with synthetic data generation: the agent produces a notebook that generates plausible synthetic data for preview

## Intermediate representation

In it's internal XML representation, an Excel workbook is a collection of cell arrays with formulas that reference other cell arrays. In the most abstract representation, the models represented in Excel are tensors indexed by meaningful dimensions (such as year, region, cost centre) which form a computational graph
