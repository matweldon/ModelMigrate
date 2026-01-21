# ModelMigrate

This project builds an agentic AI microservice to manage (audit, document and convert) economic, policy, or accounting models contained in bloated, complex Excel workbooks.

The product will enable civil servants, consultants and other professionals who deal with complex Excel models to:

* Generate documentation for the workbook
* Convert the model into a reproducible Python representation, augmented by an interactive interface

The product will eventually be an agentic AI microservice running on a robust, asyncronous cloud architecture.
At the core of the product will be a suite of tools that parse Excel workbooks to an intermediate data structure (according to a deterministic algorithm), and an agentic harness that gives an AI agent the capabilities to manipulate the intermediate data to improve interpretability before generating outputs. The agentic harness will also run checks to verify that the outputs maintain fidelity to the inputs.

The package will have the following features:

* Anonymiser: A non-AI tool to replace all sensitive data in the workbook with placeholder values
* AI-augmented deterministic parsing algorithm: The workbook is parsed to an intermediate representation deterministically to avoid hallucinations.
The AI agent then reasons about the arrays and variables to edit the intermediate representation to optimise interpretability, before generating a Python model and other outputs
* Verifiable fidelity: the AI agent is augmented by a harness including
robust parsing tools and failsafe checks to avoid hallucinations
* Interactive notebook with synthetic data generation: the agent produces a notebook that generates plausible synthetic data for preview

## Intermediate representation

