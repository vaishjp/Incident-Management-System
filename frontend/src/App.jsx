import React, { useState, useEffect } from 'react';
import axios from 'axios';

const BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:3000";

export default function Dashboard() {
  const [incidents, setIncidents] = useState([]);
  const [selectedIncident, setSelectedIncident] = useState(null);
  const [signals, setSignals] = useState([]);
  const [loadingSignals, setLoadingSignals] = useState(false);
  const [signalError, setSignalError] = useState('');
  const [rcaData, setRcaData] = useState({ root_cause: '', fix_applied: '' });
  const [feedbackMsg, setFeedbackMsg] = useState('');

  // --- Helper ---
  const formatDate = (date) => {
    if (!date) return "N/A";
    const d = new Date(date);
    return isNaN(d.getTime()) ? "N/A" : d.toLocaleString();
  };

  // --- Fetch incidents ---
  const fetchIncidents = async () => {
    try {
      const res = await axios.get(`${BASE_URL}/incidents`);

      const severityWeight = { CRITICAL: 1, P0: 1, HIGH: 2, P1: 2, MEDIUM: 3, P2: 3, LOW: 4, P3: 4 };

      const sorted = [...res.data].sort((a, b) => {
        return (severityWeight[a.severity] || 99) - (severityWeight[b.severity] || 99);
      });

      setIncidents(sorted);
    } catch (err) {
      console.error("Failed to fetch incidents:", err);
    }
  };

  useEffect(() => {
    fetchIncidents();
    const interval = setInterval(fetchIncidents, 5000);
    return () => clearInterval(interval);
  }, []);

  // --- Fetch raw signals ---
  useEffect(() => {
    if (selectedIncident) {
      setLoadingSignals(true);
      setSignalError('');

      axios.get(`${BASE_URL}/incidents/${selectedIncident.ID}/signals`)
        .then(res => {
          setSignals(res.data);
          setLoadingSignals(false);
        })
        .catch(() => {
          setSignalError("Failed to load signals");
          setLoadingSignals(false);
        });
    } else {
      setSignals([]);
    }
  }, [selectedIncident]);

  // --- Actions ---
  const handleAcknowledge = async (id) => {
    try {
      await axios.post(`${BASE_URL}/incidents/${id}/acknowledge`);
      fetchIncidents();
      setFeedbackMsg(`✅ Incident ${id} acknowledged`);
    } catch (err) {
      setFeedbackMsg(`❌ ${err.response?.data?.error || err.message}`);
    }
  };

  const handleSubmitRCA = async (e) => {
    e.preventDefault();
    try {
      const res = await axios.post(`${BASE_URL}/rca`, {
        work_item_id: selectedIncident.ID,
        root_cause: rcaData.root_cause,
        fix_applied: rcaData.fix_applied,
        submitted_by: "SRE Admin"
      });

      setFeedbackMsg(`✅ ${res.data.status} (MTTR: ${res.data.mttr_minutes} mins)`);
      setRcaData({ root_cause: '', fix_applied: '' });
      fetchIncidents();
      setSelectedIncident({ ...selectedIncident, status: "RESOLVED" });

    } catch (err) {
      setFeedbackMsg(`❌ ${err.response?.data?.error || err.message}`);
    }
  };

  const handleClose = async (id) => {
    try {
      const res = await axios.post(`${BASE_URL}/incidents/${id}/close`);
      setFeedbackMsg(`✅ ${res.data.status}`);
      fetchIncidents();
      setSelectedIncident(null);
    } catch (err) {
      setFeedbackMsg(`❌ ${err.response?.data?.error || err.message}`);
    }
  };

  return (
    <div style={{ display: 'flex', padding: '20px', gap: '20px', fontFamily: 'system-ui' }}>

      {/* LEFT PANEL */}
      <div style={{ flex: 1 }}>
        <h2>🚨 Active Incidents</h2>

        {feedbackMsg && (
          <div style={{ marginBottom: '10px', padding: '10px', background: '#eef6ff' }}>
            {feedbackMsg}
          </div>
        )}

        {incidents.length === 0 ? <p>No incidents</p> : incidents.map(inc => (
          <div
            key={inc.ID}
            onClick={() => setSelectedIncident(inc)}
            style={{
              border: selectedIncident?.ID === inc.ID ? '2px solid blue' : '1px solid gray',
              padding: '15px',
              marginBottom: '10px',
              borderRadius: '8px',
              cursor: 'pointer'
            }}
          >
            <h3>{inc.component_id} - {inc.severity}</h3>
            <p><strong>Type:</strong> {inc.error_type}</p>
            <p><strong>Status:</strong> {inc.status}</p>
            <p><strong>Time:</strong> {formatDate(inc.first_signal_time)}</p>

            {inc.status === 'OPEN' && (
              <button onClick={(e) => { e.stopPropagation(); handleAcknowledge(inc.ID); }}>
                Acknowledge
              </button>
            )}

            {inc.status === 'RESOLVED' && (
              <button onClick={(e) => { e.stopPropagation(); handleClose(inc.ID); }}>
                Close
              </button>
            )}
          </div>
        ))}
      </div>

      {/* RIGHT PANEL */}
      <div style={{ flex: 1, borderLeft: '2px solid #ddd', paddingLeft: '20px' }}>
        {selectedIncident ? (
          <>
            <h2>Incident #{selectedIncident.ID}</h2>

            {selectedIncident.status === 'INVESTIGATING' && (
              <form onSubmit={handleSubmitRCA}>
                <textarea
                  placeholder="Root Cause"
                  value={rcaData.root_cause}
                  onChange={e => setRcaData({ ...rcaData, root_cause: e.target.value })}
                  required
                />
                <textarea
                  placeholder="Fix Applied"
                  value={rcaData.fix_applied}
                  onChange={e => setRcaData({ ...rcaData, fix_applied: e.target.value })}
                  required
                />
                <button type="submit">Submit RCA</button>
              </form>
            )}

            <h3>📡 Raw Signals</h3>
            <div style={{ background: '#222', color: '#0f0', padding: '10px', maxHeight: '400px', overflowY: 'auto' }}>
              {loadingSignals ? (
                <p>Loading...</p>
              ) : signalError ? (
                <p>{signalError}</p>
              ) : signals.length === 0 ? (
                <p>No signals</p>
              ) : (
                signals.map((s, i) => (
                  <div key={i}>
                    [{formatDate(s.timestamp)}] {s.error_type}: {s.message}
                  </div>
                ))
              )}
            </div>
          </>
        ) : (
          <p>Select an incident</p>
        )}
      </div>
    </div>
  );
}
