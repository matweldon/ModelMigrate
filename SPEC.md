# ModelMigrate

The purpose of this Python package is to take an economic, policy, or accounting model contained in a bloated, complex Excel workbook, and convert it
into a reproducible Python notebook. This Python package will eventually form part of an agentic AI microservice.

The package will have the following features:

* Anonymiser: A non-AI tool to replace all sensitive data in the workbook with placeholder values
* AI-augmented deterministic parsing algorithm: The workbook is parsed to an intermediate representation deterministically to avoid hallucinations.
The AI agent then reasons about the arrays and variables to represent the model as a clear, usable notebook
* Guaranteed not to miss any parts of the model: the AI agent is augmented by a harness including
robust parsing tools and failsafe checks to avoid hallucinations
* Interactive notebook with synthetic data generation: the agent produces a notebook that generates plausible synthetic data for preview

