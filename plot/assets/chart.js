const LIMIT_TYPE = "LIMIT";
const MARKET_TYPE = "MARKET";
const STOP_LOSS_TYPE = "STOP_LOSS";
const LIMIT_MAKER_TYPE = "LIMIT_MAKER";

const SELL_SIDE = "SELL";
const BUY_SIDE = "BUY";

const STATUS_FILLED = "FILLED";

function unpack(rows, key) {
  return rows.map(function (row) {
    return row[key];
  });
}

document.addEventListener("DOMContentLoaded", function () {
  const params = new URLSearchParams(window.location.search);
  const pair = params.get("pair") || "";
  fetch("/data?pair=" + pair)
    .then((data) => data.json())
    .then((data) => {
      const candleStickData = {
        name: "Candles",
        x: unpack(data.candles, "time"),
        close: unpack(data.candles, "close"),
        open: unpack(data.candles, "open"),
        low: unpack(data.candles, "low"),
        high: unpack(data.candles, "high"),
        type: "candlestick",
        xaxis: "x1",
        yaxis: "y2",
      };

      const equityData = {
        name: `Equity (${data.quote})`,
        x: unpack(data.equity_values, "time"),
        y: unpack(data.equity_values, "value"),
        mode: "lines",
        fill: "tozeroy",
        xaxis: "x1",
        yaxis: "y1",
      };

      const assetData = {
        name: `Position (${data.asset}/${data.quote})`,
        x: unpack(data.asset_values, "time"),
        y: unpack(data.asset_values, "value"),
        mode: "lines",
        fill: "tozeroy",
        xaxis: "x1",
        yaxis: "y1",
      };

      const points = [];
      const annotations = [];
      data.candles.forEach((candle) => {
        candle.orders
          .filter((o) => o.status === STATUS_FILLED)
          .forEach((order) => {
            const point = {
              time: candle.time,
              position: order.price,
              side: order.side,
              color: "green",
            };
            if (order.side === SELL_SIDE) {
              point.color = "red";
            }
            points.push(point);

            const annotation = {
              x: candle.time,
              y: candle.low,
              xref: "x1",
              yref: "y2",
              text: "B",
              hovertext: `${order.updated_at}
                        <br>ID: ${order.id}
                        <br>Price: ${order.price.toLocaleString()}
                        <br>Size: ${order.quantity
                          .toPrecision(4)
                          .toLocaleString()}<br>Type: ${order.type}<br>${
                (order.profit &&
                  "Profit: " +
                    +(order.profit * 100).toPrecision(2).toLocaleString() +
                    "%") ||
                ""
              }`,
              showarrow: true,
              arrowcolor: "green",
              valign: "bottom",
              borderpad: 4,
              arrowhead: 2,
              ax: 0,
              ay: 20,
              font: {
                size: 12,
                color: "green",
              },
            };

            if (order.side === SELL_SIDE) {
              annotation.font.color = "red";
              annotation.arrowcolor = "red";
              annotation.text = "S";
              annotation.y = candle.high;
              annotation.ay = -20;
              annotation.valign = "top";
            }

            annotations.push(annotation);
          });
      });

      const shapes = data.shapes.map((s) => {
        return {
          type: "rect",
          xref: "x1",
          yref: "y2",
          yaxis: "y2",
          xaxis: "x1",
          x0: s.x0,
          y0: s.y0,
          x1: s.x1,
          y1: s.y1,
          line: {
            width: 0,
          },
          fillcolor: s.color,
        };
      });

      // max draw down
      if (data.max_drawdown) {
        const topPosition = data.equity_values.reduce((p, v) => {
          return p > v.value ? p : v.value;
        });
        shapes.push({
          type: "rect",
          xref: "x1",
          yref: "y1",
          yaxis: "y1",
          xaxis: "x1",
          x0: data.max_drawdown.start,
          y0: 0,
          x1: data.max_drawdown.end,
          y1: topPosition,
          line: {
            width: 0,
          },
          fillcolor: "rgba(255,0,0,0.2)",
          layer: "below",
        });

        const annotationPosition = new Date(
          (new Date(data.max_drawdown.start).getTime() +
            new Date(data.max_drawdown.end).getTime()) /
            2
        );

        annotations.push({
          x: annotationPosition,
          y: topPosition / 2.0,
          xref: "x1",
          yref: "y1",
          text: `Drawdown<br>${data.max_drawdown.value}%`,
          showarrow: false,
          font: {
            size: 12,
            color: "red",
          },
        });
      }

      const sellPoints = points.filter((p) => p.side === SELL_SIDE);
      const buyPoints = points.filter((p) => p.side === BUY_SIDE);
      const buyData = {
        name: "Buy Points",
        x: unpack(buyPoints, "time"),
        y: unpack(buyPoints, "position"),
        xaxis: "x1",
        yaxis: "y2",
        mode: "markers",
        type: "scatter",
        marker: {
          color: "green",
        },
      };
      const sellData = {
        name: "Sell Points",
        x: unpack(sellPoints, "time"),
        y: unpack(sellPoints, "position"),
        xaxis: "x1",
        yaxis: "y2",
        mode: "markers",
        type: "scatter",
        marker: {
          color: "red",
        },
      };

      const standaloneIndicators = data.indicators.reduce(
        (total, indicator) => {
          if (!indicator.overlay) {
            return total + 1;
          }
          return total;
        },
        0
      );

      let layout = {
        template: "ggplot2",
        dragmode: "zoom",
        margin: {
          t: 25,
        },
        showlegend: true,
        xaxis: {
          autorange: true,
          rangeslider: { visible: false },
          showline: true,
          anchor: standaloneIndicators > 0 ? "y3" : "y2",
        },
        yaxis2: {
          domain: standaloneIndicators > 0 ? [0.4, 0.9] : [0, 0.9],
          autorange: true,
          mirror: true,
          showline: true,
          gridcolor: "#ddd",
        },
        yaxis1: {
          domain: [0.9, 1],
          autorange: true,
          mirror: true,
          showline: true,
          gridcolor: "#ddd",
        },
        hovermode: "x unified",
        annotations: annotations,
        shapes: shapes,
      };

      let plotData = [
        candleStickData,
        equityData,
        assetData,
        buyData,
        sellData,
      ];

      const indicatorsHeight = 0.39 / standaloneIndicators;
      let standaloneIndicatorIndex = 0;
      data.indicators.forEach((indicator) => {
        const axisNumber = standaloneIndicatorIndex + 3;
        if (!indicator.overlay) {
          const heightStart = standaloneIndicatorIndex * indicatorsHeight;
          layout["yaxis" + axisNumber] = {
            title: indicator.name,
            domain: [heightStart, heightStart + indicatorsHeight],
            autorange: true,
            mirror: true,
            showline: true,
            linecolor: "black",
            gridcolor: "#ddd",
          };
          standaloneIndicatorIndex++;
        }

        indicator.metrics.forEach((metric) => {
          const data = {
            title: indicator.name,
            name: indicator.name + (metric.name && " - " + metric.name),
            x: metric.time,
            y: metric.value,
            type: metric.style,
            line: {
              color: metric.color,
            },
            xaxis: "x1",
            yaxis: "y2",
          };
          if (!indicator.overlay) {
            data.yaxis = "y" + axisNumber;
          }
          plotData.push(data);
        });
      });
      Plotly.newPlot("graph", plotData, layout);
    });
});
