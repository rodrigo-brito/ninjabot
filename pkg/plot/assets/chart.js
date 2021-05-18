function unpack(rows, key) {
  return rows.map(function (row) {
    return row[key];
  });
}

document.addEventListener("DOMContentLoaded", function () {
  const candleByDate = {};
  candles.forEach((candle) => {
    candleByDate[candle.Time] = candle;
  });
  const candleStickData = {
    x: unpack(candles, "Time"),
    close: unpack(candles, "Close"),
    open: unpack(candles, "Open"),
    low: unpack(candles, "Low"),
    high: unpack(candles, "High"),
    type: "candlestick",
    xaxis: "x",
    yaxis: "y",
  };

  var layout = {
    dragmode: "zoom",
    margin: {
      r: 10,
      t: 25,
      b: 40,
      l: 60,
    },
    showlegend: false,
    xaxis: {
      autorange: true,
    },
    yaxis: {
      autorange: true,
      type: "linear",
    },
    annotations: orders.map((order) => {
      const annotation = {
        x: order.Date,
        y: candleByDate[order.Date].Low,
        xref: "x",
        yref: "y",
        text: "B",
        hovertext: `ID: ${order.ID}<br>Size: ${order.Quantity.toPrecision(
          4
        )}<br>Type: ${order.Type}<br>${
          (order.Profit &&
            "Profit: " + (order.Profit * 100).toPrecision(2) + "%") ||
          ""
        }`,
        showarrow: true,
        arrowcolor: "green",
        valign: "bottom",
        borderpad: 4,
        arrowhead: 2,
        ax: 0,
        ay: 15,
        font: {
          size: 12,
          color: "green",
        },
      };

      if (order.Side === "SELL") {
        annotation.font.color = "red";
        annotation.arrowcolor = "red";
        annotation.text = "S";
        annotation.y = candleByDate[order.Date].High;
        annotation.ay = -15;
      }

      return annotation;
    }),
  };

  Plotly.newPlot("graph", [candleStickData], layout);
});
