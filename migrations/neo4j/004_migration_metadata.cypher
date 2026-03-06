// Program execution mode index
CREATE INDEX program_executionMode IF NOT EXISTS FOR (p:Program) ON (p.executionMode);

// Program risk score index
CREATE INDEX program_riskScore IF NOT EXISTS FOR (p:Program) ON (p.riskScore);

// ExternalInterface indexes
CREATE INDEX ext_iface_type IF NOT EXISTS FOR (e:ExternalInterface) ON (e.type);
CREATE INDEX ext_iface_programId IF NOT EXISTS FOR (e:ExternalInterface) ON (e.programId);

// Paragraph error pattern index
CREATE INDEX paragraph_errorPattern IF NOT EXISTS FOR (p:Paragraph) ON (p.errorPattern);

// Program volume estimate index
CREATE INDEX program_volumeEstimate IF NOT EXISTS FOR (p:Program) ON (p.volumeEstimate);
